package webview

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

type fakePDFDialog struct {
	reply  func(PDFDestination, error)
	shown  atomic.Int32
	closed atomic.Int32
}

func (d *fakePDFDialog) Show()  { d.shown.Add(1) }
func (d *fakePDFDialog) Close() { d.closed.Add(1) }

type pdfTestWindow struct {
	pdf         *PDFExporter
	termination *terminationCoordinator
	queue       chan func()
	dialogs     chan *fakePDFDialog
}

func newPDFTestWindow(t *testing.T) *pdfTestWindow {
	t.Helper()
	if !nativePDFSupported {
		t.Skip("native adapter is Linux-only")
	}
	p := NewPDFExporter(PDFOptions{})
	if err := p.claim(); err != nil {
		t.Fatal(err)
	}
	c := newTerminationCoordinator()
	c.onEnding = p.cancelWindow
	w := &pdfTestWindow{p, c, make(chan func(), 16), make(chan *fakePDFDialog, 16)}
	c.bind(func(fn func()) { w.queue <- fn }, func() {})
	p.bind(c.enqueue, func(_ PDFDestinationRequest, reply func(PDFDestination, error)) (nativePDFDialog, error) {
		d := &fakePDFDialog{reply: reply}
		w.dialogs <- d
		return d, nil
	})
	c.ready()
	p.markReady()
	t.Cleanup(func() { c.end(); p.endUI() })
	return w
}
func (w *pdfTestWindow) step(t *testing.T) {
	t.Helper()
	select {
	case fn := <-w.queue:
		fn()
	case <-time.After(time.Second):
		t.Fatal("no native work queued")
	}
}
func awaitPDF[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(2 * time.Second):
		t.Fatal("operation did not finish")
		var v T
		return v
	}
}
func requirePDFCode(t *testing.T, err error, code PDFErrorCode) {
	t.Helper()
	var e *PDFError
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("wanted %s, got %v", code, err)
	}
}

func TestPDFReadinessAndStartupFailure(t *testing.T) {
	p := NewPDFExporter(PDFOptions{})
	_, err := p.Render(context.Background(), PDFDocument{})
	if !nativePDFSupported {
		requirePDFCode(t, err, PDFUnsupported)
		return
	}
	requirePDFCode(t, err, PDFNotReady)
	err = Run(App{PDF: p, Listen: func() (*http.Server, net.Listener, error) { return nil, nil, errors.New("fixture listen failure") }})
	if err == nil || p.WindowContext().Err() == nil {
		t.Fatal("startup failure did not cancel window lifetime")
	}
	if err := p.claim(); err == nil {
		t.Fatal("closed handle rebound")
	}
	p = NewPDFExporter(PDFOptions{})
	t.Setenv("DFW_DAEMON_ADDR", "not-an-address")
	if err := Window(WindowApp{PDF: p}); err == nil || p.WindowContext().Err() == nil {
		t.Fatal("window discovery failure did not cancel lifetime")
	}
}

func TestPDFChoosePreservesNativeResults(t *testing.T) {
	for _, choice := range []PDFDestination{{Path: "/tmp/report"}, {Path: "/tmp/report.txt"}, {Path: "/tmp/report.pdf"}, {Cancelled: true}} {
		t.Run(choice.Path, func(t *testing.T) {
			w := newPDFTestWindow(t)
			results := make(chan pdfChoiceResult, 1)
			go func() {
				d, err := w.pdf.Choose(context.Background(), PDFDestinationRequest{SuggestedName: "report.pdf"})
				results <- pdfChoiceResult{d, err}
			}()
			w.step(t)
			dialog := awaitPDF(t, w.dialogs)
			_, err := w.pdf.Render(context.Background(), PDFDocument{})
			requirePDFCode(t, err, PDFBusy)
			dialog.reply(choice, nil)
			got := awaitPDF(t, results)
			if got.err != nil || got.destination != choice {
				t.Fatalf("native name changed: %+v", got)
			}
			dialog.reply(PDFDestination{Path: "/tmp/stale"}, nil)
			if dialog.closed.Load() != 1 {
				t.Fatal("dialog completed more than once")
			}
			w.pdf.render = func(context.Context, string, PDFDocument) ([]byte, error) { return []byte("pdf"), nil }
			if _, err := w.pdf.Render(context.Background(), PDFDocument{}); err != nil {
				t.Fatalf("chooser left capability busy: %v", err)
			}
		})
	}
}

func TestPDFChooseCancellationAndStaleDispatch(t *testing.T) {
	for _, shown := range []bool{false, true} {
		t.Run(map[bool]string{false: "queued", true: "shown"}[shown], func(t *testing.T) {
			w := newPDFTestWindow(t)
			done := make(chan error, 1)
			go func() {
				_, err := w.pdf.Choose(context.Background(), PDFDestinationRequest{SuggestedName: "report.pdf"})
				done <- err
			}()
			var dialog *fakePDFDialog
			if shown {
				w.step(t)
				dialog = awaitPDF(t, w.dialogs)
			} else {
				fn := awaitPDF(t, w.queue)
				w.queue <- fn
			}
			w.termination.request()
			w.termination.end()
			w.pdf.endUI()
			if err := awaitPDF(t, done); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			for len(w.queue) > 0 {
				(<-w.queue)()
			}
			if shown && dialog.closed.Load() != 1 {
				t.Fatal("native dialog not disposed")
			}
			if !shown && len(w.dialogs) != 0 {
				t.Fatal("stale dispatch created a dialog")
			}
		})
	}
}

func TestPDFCallerCancellationDismissesChooser(t *testing.T) {
	w := newPDFTestWindow(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := w.pdf.Choose(ctx, PDFDestinationRequest{SuggestedName: "report.pdf"}); done <- err }()
	w.step(t)
	dialog := awaitPDF(t, w.dialogs)
	cancel()
	if err := awaitPDF(t, done); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	w.step(t)
	if dialog.closed.Load() != 1 {
		t.Fatal("cancellation did not dismiss chooser")
	}
}

func TestPDFWindowLifetimeOutlivesRender(t *testing.T) {
	w := newPDFTestWindow(t)
	w.pdf.render = func(context.Context, string, PDFDocument) ([]byte, error) { return []byte("pdf"), nil }
	if _, err := w.pdf.Render(context.Background(), PDFDocument{}); err != nil {
		t.Fatal(err)
	}
	if w.pdf.WindowContext().Err() != nil {
		t.Fatal("render ended window lifetime")
	}
	requests := make(chan *CloseRequest, 2)
	controller := newCloseController(func(r *CloseRequest) { requests <- r }, w.termination.request)
	controller.requestClose()
	request := awaitPDF(t, requests)
	if w.pdf.WindowContext().Err() != nil {
		t.Fatal("pending close cancelled lifetime")
	}
	request.KeepOpen()
	if w.pdf.WindowContext().Err() != nil {
		t.Fatal("declined close cancelled lifetime")
	}
	controller.requestClose()
	awaitPDF(t, requests).Close()
	if w.pdf.WindowContext().Err() == nil {
		t.Fatal("accepted close did not cancel synchronously")
	}
	_, err := w.pdf.Render(context.Background(), PDFDocument{})
	requirePDFCode(t, err, PDFClosed)
}

func TestPDFRenderCancellationDrainsBeforeEnd(t *testing.T) {
	w := newPDFTestWindow(t)
	started := make(chan struct{})
	cleaned := make(chan struct{})
	w.pdf.render = func(ctx context.Context, _ string, _ PDFDocument) ([]byte, error) {
		close(started)
		<-ctx.Done()
		close(cleaned)
		return nil, ctx.Err()
	}
	done := make(chan error, 1)
	go func() { _, err := w.pdf.Render(context.Background(), PDFDocument{}); done <- err }()
	awaitPDF(t, started)
	w.termination.request()
	w.termination.end()
	w.pdf.endUI()
	select {
	case <-cleaned:
	default:
		t.Fatal("window ended before renderer cleanup")
	}
	if err := awaitPDF(t, done); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestPDFEarlyTerminationCancelsLifetime(t *testing.T) {
	p := NewPDFExporter(PDFOptions{})
	if err := p.claim(); err != nil {
		t.Fatal(err)
	}
	c := newTerminationCoordinator()
	c.onEnding = p.cancelWindow
	c.request()
	if p.WindowContext().Err() == nil {
		t.Fatal("early termination lost window cancellation")
	}
	c.bind(func(func()) { t.Fatal("early termination queued native work") }, func() {})
	c.ready()
	p.markReady()
	p.endUI()
}

func TestPDFChooserFailureReleasesOperation(t *testing.T) {
	w := newPDFTestWindow(t)
	w.pdf.factory = func(PDFDestinationRequest, func(PDFDestination, error)) (nativePDFDialog, error) {
		return nil, errors.New("fixture chooser failure")
	}
	done := make(chan error, 1)
	go func() {
		_, err := w.pdf.Choose(context.Background(), PDFDestinationRequest{SuggestedName: "report.pdf"})
		done <- err
	}()
	w.step(t)
	if err := awaitPDF(t, done); err == nil || err.Error() != "fixture chooser failure" {
		t.Fatal(err)
	}
	w.pdf.render = func(context.Context, string, PDFDocument) ([]byte, error) {
		return nil, errors.New("fixture render failure")
	}
	if _, err := w.pdf.Render(context.Background(), PDFDocument{}); err == nil || err.Error() != "fixture render failure" {
		t.Fatal(err)
	}
	if _, err := w.pdf.Choose(context.Background(), PDFDestinationRequest{SuggestedName: "../bad"}); err == nil {
		t.Fatal("bad basename accepted")
	}
}

func TestPDFRebindDoesNotCancelOriginalWindow(t *testing.T) {
	w := newPDFTestWindow(t)
	if err := Run(App{PDF: w.pdf}); err == nil {
		t.Fatal("live handle was rebound")
	}
	if w.pdf.WindowContext().Err() != nil {
		t.Fatal("failed rebind cancelled the original window")
	}
}
