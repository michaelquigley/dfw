//go:build linux

package pdf

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const shutdownGrace = 2 * time.Second

// tail bounds untrusted stderr. it is never included in a returned error: a
// browser can print URLs containing the caller's ephemeral credentials.
type tail struct {
	mu   sync.Mutex
	data []byte
}

func (t *tail) Write(data []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := len(data)
	if len(data) > 16<<10 {
		data = data[len(data)-(16<<10):]
	}
	t.data = append(t.data, data...)
	if len(t.data) > 16<<10 {
		t.data = append([]byte(nil), t.data[len(t.data)-(16<<10):]...)
	}
	return n, nil
}

type process struct {
	cmd      *exec.Cmd
	pipe     *pipe
	profile  string
	done     chan struct{}
	stopOnce sync.Once
	stopErr  error
}

func executable(override string) (string, error) {
	if override != "" {
		p, err := exec.LookPath(override)
		if err != nil {
			return "", Failure(Unavailable, "configured Chromium executable is unavailable")
		}
		return p, nil
	}
	for _, name := range []string{"chromium", "chromium-browser", "/snap/bin/chromium"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", Failure(Unavailable, "install system Chromium or configure its executable to export PDF")
}

func startProcess(override string) (*process, error) {
	bin, err := executable(override)
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, Failure(RenderFailed, "resolve home directory for the private browser profile")
	}
	profile, err := os.MkdirTemp(home, "dfw-pdf-*")
	if err != nil {
		return nil, Failure(RenderFailed, "create private browser profile in the home directory")
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(profile)
		}
	}()
	childIn, parentOut, err := os.Pipe()
	if err != nil {
		return nil, Failure(RenderFailed, "create browser control pipe")
	}
	defer func() { _ = childIn.Close() }()
	childOutParent, childOut, err := os.Pipe()
	if err != nil {
		_ = parentOut.Close()
		return nil, Failure(RenderFailed, "create browser reply pipe")
	}
	defer func() { _ = childOut.Close() }()
	args := []string{"--headless", "--remote-debugging-pipe", "--user-data-dir=" + filepath.Clean(profile), "--no-first-run", "--no-default-browser-check", "--disable-background-networking", "--disable-extensions", "--disable-component-extensions-with-background-pages", "--no-startup-window"}
	cmd := exec.Command(bin, args...)
	cmd.ExtraFiles = []*os.File{childIn, childOut}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stderr = &tail{}
	cmd.WaitDelay = shutdownGrace
	if err := cmd.Start(); err != nil {
		_ = parentOut.Close()
		_ = childOutParent.Close()
		return nil, Failure(Unavailable, "could not start the configured Chromium executable")
	}
	p := &process{cmd: cmd, pipe: newPipe(childOutParent, parentOut), profile: profile, done: make(chan struct{})}
	go func() { _ = cmd.Wait(); close(p.done) }()
	success = true
	return p, nil
}

// stop reaps only this job's process group, even when its launcher exited before
// the browser. pipes close before escalation so a stuck reader/writer releases.
func (p *process) stop() error {
	p.stopOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		_, _ = p.pipe.call(ctx, "Browser.close", nil, "")
		p.pipe.close()
		select {
		case <-p.done:
		case <-ctx.Done():
		}
		cancel()
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGTERM)
		deadline := time.NewTimer(shutdownGrace)
		tick := time.NewTicker(10 * time.Millisecond)
		defer deadline.Stop()
		defer tick.Stop()
	waitGroup:
		for groupExists(p.cmd.Process.Pid) {
			select {
			case <-deadline.C:
				break waitGroup
			case <-tick.C:
			}
		}
		if groupExists(p.cmd.Process.Pid) {
			_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		}
		<-p.done
		if err := os.RemoveAll(p.profile); err != nil {
			p.stopErr = Failure(RenderFailed, "could not remove the private browser profile")
		}
	})
	return p.stopErr
}

func groupExists(pid int) bool { return !errors.Is(syscall.Kill(-pid, 0), syscall.ESRCH) }
