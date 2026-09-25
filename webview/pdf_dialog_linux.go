//go:build linux

package webview

/*
#cgo pkg-config: gtk+-3.0
#include <stdlib.h>
#include "pdf_dialog_linux.h"
*/
import "C"

import (
	"path/filepath"
	"runtime/cgo"
	"unsafe"

	"github.com/michaelquigley/dfw/internal/pdf"
)

const nativePDFSupported = true

type linuxPDFDialog struct {
	dialog *C.dfw_pdf_dialog
	handle cgo.Handle
}

func nativePDFDialogFactory(parent unsafe.Pointer) pdfDialogFactory {
	return func(request PDFDestinationRequest, reply func(PDFDestination, error)) (nativePDFDialog, error) {
		if !validNativeWindow(parent) {
			return nil, pdf.Failure(pdf.ChooserFailed, "no native parent for the PDF chooser")
		}
		name, directory := C.CString(request.SuggestedName), C.CString(request.InitialDirectory)
		defer C.free(unsafe.Pointer(name))
		defer C.free(unsafe.Pointer(directory))
		handle := cgo.NewHandle(reply)
		dialog := C.dfw_pdf_dialog_new((*C.GtkWindow)(parent), name, directory, C.uintptr_t(handle))
		if dialog == nil {
			handle.Delete()
			return nil, pdf.Failure(pdf.ChooserFailed, "could not create the native PDF chooser")
		}
		return &linuxPDFDialog{dialog: dialog, handle: handle}, nil
	}
}

func (d *linuxPDFDialog) Show() { C.dfw_pdf_dialog_show(d.dialog) }
func (d *linuxPDFDialog) Close() {
	if d.dialog == nil {
		return
	}
	C.dfw_pdf_dialog_free(d.dialog)
	d.dialog = nil
	d.handle.Delete()
}

//export dfwPDFChosen
func dfwPDFChosen(handle C.uintptr_t, accepted C.int, path *C.char) {
	reply := cgo.Handle(handle).Value().(func(PDFDestination, error))
	if accepted == 0 {
		reply(PDFDestination{Cancelled: true}, nil)
		return
	}
	chosen := C.GoString(path)
	if chosen == "" || !filepath.IsAbs(chosen) {
		reply(PDFDestination{}, pdf.Failure(pdf.ChooserFailed, "the chooser did not return a local absolute path"))
		return
	}
	reply(PDFDestination{Path: chosen}, nil)
}
