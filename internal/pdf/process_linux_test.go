//go:build linux

package pdf

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/michaelquigley/df/dd"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv("DFW_PDF_TEST_CHILD"); mode != "" {
		fakeBrowser(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// fakeBrowser runs in the test executable as a real child, using descriptors 3/4.
func fakeBrowser(mode string) {
	in, out := os.NewFile(3, "commands"), os.NewFile(4, "replies")
	if in == nil || out == nil {
		os.Exit(2)
	}
	if mode == "tree" {
		// the launcher swallows termination; only signalling the whole group
		// reaches the child and allows the launcher to reap it and exit.
		signal.Notify(make(chan os.Signal, 1), syscall.SIGTERM)
		self, _ := os.Executable()
		_ = os.Setenv("DFW_PDF_TEST_CHILD", "term-only")
		child := exec.Command(self)
		child.ExtraFiles = []*os.File{in, out}
		_ = child.Run()
		return
	}
	if mode == "term-only" {
		term := make(chan os.Signal, 1)
		signal.Notify(term, syscall.SIGTERM)
		go func() { <-term; os.Exit(0) }()
	}
	if mode == "stubborn" {
		signal.Ignore(syscall.SIGTERM)
	}
	scanner := bufio.NewScanner(in)
	scanner.Split(splitMessage)
	for scanner.Scan() {
		var message wireMessage
		if dd.BindJSON(&message, scanner.Bytes()) != nil {
			os.Exit(3)
		}
		method := text(message.Fields, "method")
		if mode == "exit" {
			os.Exit(4)
		}
		if mode == "malformed" {
			_, _ = out.Write([]byte("not json\x00"))
			continue
		}
		if method == "Browser.close" && (mode == "stubborn" || mode == "term-only") {
			continue
		}
		if method == os.Getenv("DFW_PDF_TEST_FAIL") {
			data, _ := dd.UnbindJSON(wireMessage{Fields: map[string]any{"id": message.Fields["id"], "error": map[string]any{"message": "secret document URL"}}})
			_, _ = out.Write(append(data, 0))
			continue
		}
		result := map[string]any{}
		if mode == "engine" {
			switch method {
			case "Target.createTarget":
				result["targetId"] = "page"
			case "Target.attachToTarget":
				result["sessionId"] = "session"
			case "Page.navigate":
				result["loaderId"], result["frameId"] = "loader", "frame"
			case "Page.createIsolatedWorld":
				result["executionContextId"] = 7
			case "Runtime.evaluate":
				result["result"] = map[string]any{"value": true}
			case "Page.printToPDF":
				result["stream"] = "stream"
			case "IO.read":
				result["data"], result["eof"] = "%PDF-fixture", true
			}
		}
		if method == "Browser.getVersion" {
			result["product"] = "Chrome/153.0"
			result["pid"] = os.Getpid()
		}
		data, err := dd.UnbindJSON(wireMessage{Fields: map[string]any{"id": message.Fields["id"], "result": result}})
		if err != nil {
			os.Exit(5)
		}
		_, _ = out.Write(append(data, 0))
		if mode == "engine" && method == "Page.navigate" && os.Getenv("DFW_PDF_TEST_NO_LOAD") != "1" {
			event, _ := dd.UnbindJSON(wireMessage{Fields: map[string]any{"method": "Page.lifecycleEvent", "sessionId": "session", "params": map[string]any{"loaderId": "loader", "name": "load"}}})
			_, _ = out.Write(append(event, 0))
		}
		if method == "Browser.close" {
			return
		}
	}
	if mode == "stubborn" || mode == "term-only" {
		for {
			time.Sleep(time.Hour)
		}
	}
}

func TestProcessPipesAndCleanup(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"normal", "stubborn", "tree", "exit", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DFW_PDF_TEST_CHILD", mode)
			home := t.TempDir()
			t.Setenv("HOME", home)
			p, err := startProcess(self)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = p.stop() })
			if info, err := os.Stat(p.profile); err != nil || info.Mode().Perm() != 0700 {
				t.Fatalf("profile permissions: %v %v", info, err)
			}
			if filepath.Dir(p.profile) != home {
				t.Fatal("profile outside configured home")
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			result, err := p.pipe.call(ctx, "Browser.getVersion", nil, "")
			if mode == "normal" || mode == "stubborn" || mode == "tree" {
				if err != nil || text(result, "product") != "Chrome/153.0" {
					t.Fatalf("%v %v", result, err)
				}
			} else if err == nil {
				t.Fatal("child failure accepted")
			}
			started := time.Now()
			if err := p.stop(); err != nil {
				t.Fatal(err)
			}
			if time.Since(started) > 6*time.Second {
				t.Fatal("shutdown was not bounded")
			}
			select {
			case <-p.done:
			default:
				t.Fatal("child was not reaped")
			}
			if _, err := os.Stat(p.profile); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("profile left: %v", err)
			}
			if groupExists(p.cmd.Process.Pid) {
				t.Fatal("process group left behind")
			}
			for _, arg := range p.cmd.Args {
				if arg == "--no-sandbox" || strings.HasPrefix(arg, "--remote-debugging-port") {
					t.Fatalf("unsafe argument %s", arg)
				}
			}
		})
	}
}

func TestRendererProtocolFailuresAndTimeout(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DFW_PDF_TEST_CHILD", "engine")
	for _, method := range []string{"", "Browser.getVersion", "Target.createTarget", "Target.attachToTarget", "Page.enable", "Network.enable", "Fetch.enable", "Page.navigate", "Page.createIsolatedWorld", "Runtime.evaluate", "Page.printToPDF", "IO.read"} {
		t.Run(method, func(t *testing.T) {
			t.Setenv("DFW_PDF_TEST_FAIL", method)
			home := t.TempDir()
			t.Setenv("HOME", home)
			data, err := Render(context.Background(), self, Document{URL: "http://127.0.0.1:43123/document"})
			if method == "" {
				if err != nil || string(data) != "%PDF-fixture" {
					t.Fatalf("%q %v", data, err)
				}
			} else if err == nil || len(data) != 0 || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe failure: %q %v", data, err)
			}
			entries, _ := os.ReadDir(home)
			if len(entries) != 0 {
				t.Fatal("profile left after protocol result")
			}
		})
	}
	t.Setenv("DFW_PDF_TEST_NO_LOAD", "1")
	t.Setenv("HOME", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := Render(ctx, self, Document{URL: "http://127.0.0.1:43123/document"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout: %v", err)
	}
}

func TestUnavailableAndBoundedDiagnostics(t *testing.T) {
	_, err := executable("/not/a/browser")
	var classified *Error
	if !errors.As(err, &classified) || classified.Code != Unavailable {
		t.Fatal(err)
	}
	var tail tail
	data := strings.Repeat("a", 1<<20) + "last"
	if n, err := fmt.Fprint(&tail, data); err != nil || n != len(data) {
		t.Fatal(n, err)
	}
	if len(tail.data) != 16<<10 || !strings.HasSuffix(string(tail.data), "last") {
		t.Fatal("stderr not bounded")
	}
}
