package pdf

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/michaelquigley/df/dd"
)

func encode(t *testing.T, fields map[string]any) []byte {
	t.Helper()
	data, err := dd.UnbindJSON(wireMessage{Fields: fields})
	if err != nil {
		t.Fatal(err)
	}
	return append(data, 0)
}
func testPipe(t *testing.T) (*pipe, *io.PipeReader, *io.PipeWriter) {
	t.Helper()
	incoming, childOut := io.Pipe()
	childIn, outgoing := io.Pipe()
	p := newPipe(incoming, outgoing)
	t.Cleanup(func() { _ = childIn.Close(); _ = childOut.Close(); p.close() })
	return p, childIn, childOut
}

func TestPipeInterleavedEventsAndReplies(t *testing.T) {
	p, in, out := testPipe(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 2)
	for range 2 {
		go func() {
			result, err := p.call(ctx, "method", map[string]any{"frameId": "foreign-key"}, "session")
			if err == nil && result["executionContextId"] != float64(7) {
				err = errors.New("lost foreign key")
			}
			done <- err
		}()
	}
	r := bufio.NewReader(in)
	var ids []any
	for range 2 {
		data, err := r.ReadBytes(0)
		if err != nil {
			t.Fatal(err)
		}
		var m wireMessage
		if err := dd.BindJSON(&m, data[:len(data)-1]); err != nil {
			t.Fatal(err)
		}
		if m.Fields["sessionId"] != "session" || object(m.Fields, "params")["frameId"] != "foreign-key" {
			t.Fatalf("wire keys changed: %v", m.Fields)
		}
		ids = append(ids, m.Fields["id"])
	}
	_, _ = out.Write(encode(t, map[string]any{"method": "Page.lifecycleEvent", "sessionId": "session", "params": map[string]any{"loaderId": "load", "name": "load"}}))
	for _, id := range []any{ids[1], ids[0]} {
		_, _ = out.Write(encode(t, map[string]any{"id": id, "result": map[string]any{"executionContextId": 7}}))
	}
	if text(<-p.events, "sessionId") != "session" {
		t.Fatal("event session lost")
	}
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func TestPipeRejectsBadFrames(t *testing.T) {
	for _, tc := range []struct{ name, data string }{
		{"malformed", "{bad}\x00"}, {"truncated", "{\"id\":1"}, {"oversized", strings.Repeat("x", maxMessage+1)},
		{"invalid-id", "{\"id\":-1}\x00"}, {"empty", "\x00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, _, out := testPipe(t)
			go func() { _, _ = io.Copy(out, strings.NewReader(tc.data)); _ = out.Close() }()
			select {
			case <-p.done:
			case <-time.After(time.Second):
				t.Fatal("reader did not fail")
			}
			if p.failure() == nil {
				t.Fatal("missing error")
			}
		})
	}
}

func TestPipeCancellationReleasesBlockedWriter(t *testing.T) {
	p, _, _ := testPipe(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.call(ctx, "method", nil, ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := p.call(ctx, "method", nil, ""); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	p.close()
}

func TestManifestAndVersions(t *testing.T) {
	for _, u := range []string{"file:///tmp/a", "https://127.0.0.1:4/a", "http://localhost:4/a", "http://127.0.0.1/a", "http://user@127.0.0.1:4/a", "http://127.0.0.1:4/a#fragment", "http://127.0.0.1:99999/a"} {
		if _, err := (Document{URL: u}).allowlist(); err == nil {
			t.Errorf("accepted %s", u)
		}
	}
	if _, err := (Document{URL: "http://127.0.0.1:4/a", Resources: []string{"http://127.0.0.1:5/b"}}).allowlist(); err == nil {
		t.Fatal("cross-origin resource accepted")
	}
	if _, err := (Document{URL: "http://[::1]:4/a"}).allowlist(); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"Chrome/131.0", "HeadlessChrome/153.0"} {
		if err := browserVersion(version); err != nil {
			t.Fatal(err)
		}
	}
	for _, version := range []string{"Firefox/153.0", "Chrome/130.0"} {
		if err := browserVersion(version); err == nil {
			t.Fatal("version accepted")
		}
	}
}

func TestGuardRefusesUnexpectedTargetsAndResources(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	g := newGuard(nil, map[string]bool{"http://127.0.0.1:4/a": true}, "page", "session", cancel)
	for _, e := range []map[string]any{
		{"method": "Target.targetCreated", "params": map[string]any{"targetInfo": map[string]any{"targetId": "popup", "type": "page"}}},
		{"method": "Target.attachedToTarget", "params": map[string]any{"targetInfo": map[string]any{"targetId": "worker"}}},
		{"method": "Browser.downloadWillBegin"},
		{"method": "Page.frameAttached", "sessionId": "session"},
		{"method": "Fetch.requestPaused", "sessionId": "session", "params": map[string]any{"request": map[string]any{"method": "POST", "url": "http://127.0.0.1:4/a"}}},
		{"method": "Fetch.requestPaused", "sessionId": "session", "params": map[string]any{"request": map[string]any{"method": "GET", "url": "http://outside/secret"}}},
		{"method": "Fetch.requestPaused", "sessionId": "session", "params": map[string]any{"request": map[string]any{"method": "GET", "url": "http://127.0.0.1:4/a"}, "responseStatusCode": float64(302)}},
	} {
		if err := g.handle(ctx, e); err == nil {
			t.Fatalf("accepted %v", e)
		}
	}
	if err := g.handle(ctx, map[string]any{"method": "Target.attachedToTarget", "params": map[string]any{"targetInfo": map[string]any{"targetId": "page"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestProtocolErrorsDoNotEchoBrowserSecrets(t *testing.T) {
	p, in, out := testPipe(t)
	done := make(chan error, 1)
	go func() { _, err := p.call(context.Background(), "method", nil, ""); done <- err }()
	data, _ := bufio.NewReader(in).ReadBytes(0)
	var m wireMessage
	_ = dd.BindJSON(&m, bytes.TrimSuffix(data, []byte{0}))
	_, _ = out.Write(encode(t, map[string]any{"id": m.Fields["id"], "error": map[string]any{"message": "http://127.0.0.1:4/export/secret"}}))
	err := <-done
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unsafe error: %v", err)
	}
}
