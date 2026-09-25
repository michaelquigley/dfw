//go:build !linux

package webview

import (
	"github.com/michaelquigley/dfw/internal/pdf"
	"unsafe"
)

const nativePDFSupported = false

func nativePDFDialogFactory(unsafe.Pointer) pdfDialogFactory {
	return func(PDFDestinationRequest, func(PDFDestination, error)) (nativePDFDialog, error) {
		return nil, pdf.Failure(pdf.Unsupported, "PDF export is currently supported on Linux only")
	}
}
