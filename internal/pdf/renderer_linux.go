//go:build linux

package pdf

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"time"
)

const maxPDF = 128 << 20

// Render launches an isolated system Chromium for one immutable local document.
func Render(parent context.Context, executable string, document Document) (output []byte, err error) {
	allowed, err := document.allowlist()
	if err != nil {
		return nil, err
	}
	deadline, stopDeadline := context.WithTimeout(parent, 60*time.Second)
	defer stopDeadline()
	ctx, cancel := context.WithCancelCause(deadline)
	defer cancel(nil)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	proc, err := startProcess(executable)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cleanupErr := proc.stop(); cleanupErr != nil {
			output = nil
			err = errors.Join(err, cleanupErr)
		}
	}()
	stopPipe := context.AfterFunc(ctx, func() { proc.pipe.fail(context.Cause(ctx)) })
	defer stopPipe()
	// prefer the policy/cancellation cause to the incidental EOF it produces.
	defer func() {
		if err != nil && context.Cause(ctx) != nil {
			err = context.Cause(ctx)
		}
	}()
	p := proc.pipe
	version, err := p.call(ctx, "Browser.getVersion", nil, "")
	if err != nil {
		return nil, err
	}
	if err := browserVersion(text(version, "product")); err != nil {
		return nil, err
	}
	created, err := p.call(ctx, "Target.createTarget", map[string]any{"url": "about:blank"}, "")
	if err != nil {
		return nil, err
	}
	target := text(created, "targetId")
	attached, err := p.call(ctx, "Target.attachToTarget", map[string]any{"targetId": target, "flatten": true}, "")
	if err != nil {
		return nil, err
	}
	session := text(attached, "sessionId")
	if target == "" || session == "" {
		return nil, Failure(RenderFailed, "browser did not create a rendering target")
	}
	g := newGuard(p, allowed, target, session, cancel)
	guardCtx, stopGuard := context.WithCancel(ctx)
	go g.run(guardCtx)
	defer func() { stopGuard(); <-g.done }()
	commands := []struct {
		method  string
		params  map[string]any
		browser bool
	}{
		{"Target.setDiscoverTargets", map[string]any{"discover": true}, true},
		{"Browser.setDownloadBehavior", map[string]any{"behavior": "deny", "eventsEnabled": true}, true},
		{"Target.setAutoAttach", map[string]any{"autoAttach": true, "waitForDebuggerOnStart": true, "flatten": true}, false},
		{"Page.enable", nil, false},
		{"Network.enable", nil, false},
		{"Network.setCacheDisabled", map[string]any{"cacheDisabled": true}, false},
		{"Page.setLifecycleEventsEnabled", map[string]any{"enabled": true}, false},
		{"Emulation.setScriptExecutionDisabled", map[string]any{"value": true}, false},
		{"Emulation.setEmulatedMedia", map[string]any{"media": "print"}, false},
		{"Fetch.enable", map[string]any{"patterns": []map[string]any{{"urlPattern": "*", "requestStage": "Request"}, {"urlPattern": "*", "requestStage": "Response"}}}, false},
	}
	for _, command := range commands {
		s := session
		if command.browser {
			s = ""
		}
		if _, err = p.call(ctx, command.method, command.params, s); err != nil {
			return nil, err
		}
	}
	nav, err := p.call(ctx, "Page.navigate", map[string]any{"url": document.URL}, session)
	if err != nil {
		return nil, err
	}
	if text(nav, "errorText") != "" || text(nav, "loaderId") == "" {
		return nil, Failure(Load, "could not load the printable document")
	}
	if err = g.waitLoad(ctx, text(nav, "loaderId")); err != nil {
		return nil, err
	}
	world, err := p.call(ctx, "Page.createIsolatedWorld", map[string]any{"frameId": text(nav, "frameId"), "worldName": "dfw-pdf-readiness"}, session)
	if err != nil {
		return nil, err
	}
	ready, err := p.call(ctx, "Runtime.evaluate", map[string]any{
		"contextId": number(world, "executionContextId"), "awaitPromise": true, "returnByValue": true,
		"expression": `(async () => {
            await Promise.all(Array.from(document.fonts, font => font.load()));
            await document.fonts.ready;
            if (Array.from(document.fonts).some(font => font.status !== 'loaded')) throw new Error('font failed');
            await Promise.all(Array.from(document.images, image => image.decode()));
            return true;
        })()`,
	}, session)
	if err != nil {
		return nil, err
	}
	if ready["exceptionDetails"] != nil || object(ready, "result")["value"] != true {
		return nil, Failure(Load, "print fonts or images are not ready")
	}
	printed, err := p.call(ctx, "Page.printToPDF", map[string]any{
		"printBackground": true, "displayHeaderFooter": false, "preferCSSPageSize": true,
		"paperWidth": 8.5, "paperHeight": 11, "transferMode": "ReturnAsStream",
	}, session)
	if err != nil {
		return nil, err
	}
	stream := text(printed, "stream")
	if stream == "" {
		return nil, Failure(RenderFailed, "browser returned no PDF stream")
	}
	defer func() {
		closeCtx, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		_, _ = p.call(closeCtx, "IO.close", map[string]any{"handle": stream}, session)
	}()
	for {
		chunk, err := p.call(ctx, "IO.read", map[string]any{"handle": stream, "size": 256 << 10}, session)
		if err != nil {
			return nil, err
		}
		data := []byte(text(chunk, "data"))
		if chunk["base64Encoded"] == true {
			data, err = base64.StdEncoding.DecodeString(string(data))
			if err != nil {
				return nil, Failure(RenderFailed, "browser returned an invalid PDF stream")
			}
		}
		if len(output)+len(data) > maxPDF {
			return nil, Failure(RenderFailed, "PDF exceeded the 128 MiB limit")
		}
		output = append(output, data...)
		if chunk["eof"] == true {
			break
		}
	}
	if !bytes.HasPrefix(output, []byte("%PDF-")) {
		return nil, Failure(RenderFailed, "browser output is not a PDF")
	}
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	return output, nil
}
