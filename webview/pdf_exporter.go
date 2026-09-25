package webview

import (
	"context"
	"path/filepath"
	"strings"
	"sync"

	"github.com/michaelquigley/dfw/internal/pdf"
)

// PDFOptions configures discovery without starting or downloading a browser.
type PDFOptions struct{ Executable string }

// PDFDestinationRequest suggests a name, not an output format inferred from it.
type PDFDestinationRequest struct {
	SuggestedName    string
	InitialDirectory string
}

// PDFDestination preserves the exact name accepted by the native chooser.
type PDFDestination struct {
	Path      string
	Cancelled bool
}

// PDFDocument describes the caller's immutable printable document and resources.
type PDFDocument = pdf.Document

// PDFError is a classified export error; errors.Is still identifies context errors.
type PDFError = pdf.Error

// PDFErrorCode identifies failures independently of their display text.
type PDFErrorCode = pdf.Code

const (
	PDFUnsupported    = pdf.Unsupported
	PDFNotReady       = pdf.NotReady
	PDFClosed         = pdf.Closed
	PDFBusy           = pdf.Busy
	PDFUnavailable    = pdf.Unavailable
	PDFIncompatible   = pdf.Incompatible
	PDFLoad           = pdf.Load
	PDFResourcePolicy = pdf.Policy
	PDFRenderFailed   = pdf.RenderFailed
	PDFChooserFailed  = pdf.ChooserFailed
)

type nativePDFDialog interface {
	Show()
	Close()
}
type pdfDialogFactory func(PDFDestinationRequest, func(PDFDestination, error)) (nativePDFDialog, error)
type pdfChoice struct {
	ctx      context.Context
	result   chan pdfChoiceResult
	finished chan struct{}
	dialog   nativePDFDialog
	release  func()
}
type pdfChoiceResult struct {
	destination PDFDestination
	err         error
}

// PDFExporter belongs to one window. callers use it from background goroutines;
// dfw marshals chooser work to the UI thread and owns renderer cleanup.
type PDFExporter struct {
	options PDFOptions
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	claimed bool
	ready   bool
	busy    bool
	enqueue func(func()) bool
	factory pdfDialogFactory
	choice  *pdfChoice
	workers sync.WaitGroup
	render  func(context.Context, string, pdf.Document) ([]byte, error)
}

// NewPDFExporter constructs an unbound handle. pass it to exactly one Run or Window.
func NewPDFExporter(options PDFOptions) *PDFExporter {
	ctx, cancel := context.WithCancel(context.Background())
	return &PDFExporter{options: options, ctx: ctx, cancel: cancel, render: pdf.Render}
}

// Supported reports platform support, not executable availability or readiness.
func (p *PDFExporter) Supported() bool { return p != nil && nativePDFSupported }

// WindowContext stays valid after a render returns, through product publication.
func (p *PDFExporter) WindowContext() context.Context { return p.ctx }

func (p *PDFExporter) claim() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ctx == nil || p.claimed || p.ctx.Err() != nil {
		return pdf.Failure(pdf.Closed, "PDF capability cannot be rebound")
	}
	p.claimed = true
	return nil
}

func (p *PDFExporter) bind(enqueue func(func()) bool, factory pdfDialogFactory) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.enqueue, p.factory = enqueue, factory
}

func (p *PDFExporter) markReady() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ready = p.ctx.Err() == nil
}

func (p *PDFExporter) cancelWindow() {
	if p != nil && p.cancel != nil {
		p.cancel()
	}
}

func (p *PDFExporter) begin(ctx context.Context) (context.Context, func(), error) {
	if !p.Supported() {
		return nil, nil, pdf.Failure(pdf.Unsupported, "PDF export is currently supported on Linux only")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ctx == nil || p.ctx.Err() != nil {
		return nil, nil, pdf.Failure(pdf.Closed, "PDF window is closed")
	}
	if !p.ready {
		return nil, nil, pdf.Failure(pdf.NotReady, "PDF window is not ready")
	}
	if p.busy {
		return nil, nil, pdf.Failure(pdf.Busy, "a PDF operation is already active")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	p.busy = true
	p.workers.Add(1)
	op, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(p.ctx, cancel)
	release := func() {
		stop()
		cancel()
		p.mu.Lock()
		p.busy = false
		p.mu.Unlock()
		p.workers.Done()
	}
	return op, release, nil
}

func (p *PDFExporter) schedule(fn func()) bool {
	p.mu.Lock()
	enqueue := p.enqueue
	p.mu.Unlock()
	return enqueue != nil && enqueue(fn)
}

// Choose uses native overwrite handling. it never appends or changes an extension.
func (p *PDFExporter) Choose(ctx context.Context, request PDFDestinationRequest) (PDFDestination, error) {
	opCtx, release, err := p.begin(ctx)
	if err != nil {
		return PDFDestination{}, err
	}
	if request.SuggestedName == "" || filepath.Base(request.SuggestedName) != request.SuggestedName || strings.ContainsRune(request.SuggestedName, 0) || (request.InitialDirectory != "" && !filepath.IsAbs(request.InitialDirectory)) || strings.ContainsRune(request.InitialDirectory, 0) {
		release()
		return PDFDestination{}, pdf.Failure(pdf.ChooserFailed, "expected a suggested basename and an absolute initial directory")
	}
	op := &pdfChoice{ctx: opCtx, result: make(chan pdfChoiceResult, 1), finished: make(chan struct{}), release: release}
	p.mu.Lock()
	p.choice = op
	p.mu.Unlock()
	stopCancel := context.AfterFunc(opCtx, func() {
		p.schedule(func() { p.finishChoice(op, PDFDestination{}, opCtx.Err()) })
	})
	defer stopCancel()
	if !p.schedule(func() { p.showChoice(op, request) }) {
		// no native creation was queued, so this path owns no GTK object.
		p.finishChoice(op, PDFDestination{}, pdf.Failure(pdf.Closed, "PDF window is closing"))
	}
	select {
	case result := <-op.result:
		<-op.finished
		return result.destination, result.err
	case <-opCtx.Done():
		// release cancels the operation context too. a native result was
		// queued first in that case; wait for its slot to be fully released.
		select {
		case result := <-op.result:
			<-op.finished
			return result.destination, result.err
		default:
		}
		// cancellation still needs UI cleanup even if this call has returned.
		p.schedule(func() { p.finishChoice(op, PDFDestination{}, opCtx.Err()) })
		return PDFDestination{}, opCtx.Err()
	}
}

// showChoice and finishChoice run on the UI thread once a dialog can exist.
func (p *PDFExporter) showChoice(op *pdfChoice, request PDFDestinationRequest) {
	p.mu.Lock()
	if p.choice != op {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()
	if op.ctx.Err() != nil || p.ctx.Err() != nil {
		p.finishChoice(op, PDFDestination{}, context.Canceled)
		return
	}
	dialog, err := p.factory(request, func(result PDFDestination, err error) { p.finishChoice(op, result, err) })
	if err != nil {
		p.finishChoice(op, PDFDestination{}, err)
		return
	}
	op.dialog = dialog
	if op.ctx.Err() != nil || p.ctx.Err() != nil {
		p.finishChoice(op, PDFDestination{}, context.Canceled)
		return
	}
	dialog.Show()
}

func (p *PDFExporter) finishChoice(op *pdfChoice, result PDFDestination, err error) {
	p.mu.Lock()
	if p.choice != op {
		p.mu.Unlock()
		return
	}
	p.choice = nil
	p.mu.Unlock()
	if op.dialog != nil {
		op.dialog.Close()
	}
	if op.ctx.Err() != nil {
		err = op.ctx.Err()
	} else if p.ctx.Err() != nil {
		err = p.ctx.Err()
	}
	op.result <- pdfChoiceResult{result, err}
	op.release()
	close(op.finished)
}

// Render returns bytes only. the product validates and publishes its destination.
func (p *PDFExporter) Render(ctx context.Context, document PDFDocument) ([]byte, error) {
	op, release, err := p.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	data, err := p.render(op, p.options.Executable, document)
	if p.ctx.Err() != nil {
		return nil, p.ctx.Err()
	}
	if op.Err() != nil {
		return nil, op.Err()
	}
	return data, err
}

// endUI disconnects native callbacks before the window goes, then waits for the
// cancelled renderer's bounded process cleanup. it runs before webview destruction.
func (p *PDFExporter) endUI() {
	if p == nil {
		return
	}
	p.cancelWindow()
	p.mu.Lock()
	p.ready = false
	op := p.choice
	p.enqueue = nil
	p.mu.Unlock()
	if op != nil {
		p.finishChoice(op, PDFDestination{}, context.Canceled)
	}
	p.workers.Wait()
}
