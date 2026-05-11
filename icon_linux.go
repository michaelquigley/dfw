//go:build linux

package dfw

/*
#cgo pkg-config: gtk+-3.0 gdk-pixbuf-2.0

#include <gtk/gtk.h>
#include <gdk-pixbuf/gdk-pixbuf.h>
#include <stdlib.h>

static char *dfw_set_window_icon(GtkWindow *window, const unsigned char *data, size_t len) {
	if (window == NULL || data == NULL || len == 0) {
		return NULL;
	}

	GError *err = NULL;
	GdkPixbufLoader *loader = gdk_pixbuf_loader_new_with_type("png", &err);
	if (loader == NULL) {
		if (err != NULL) {
			char *message = g_strdup(err->message);
			g_error_free(err);
			return message;
		}
		return g_strdup("unable to create png loader");
	}

	if (!gdk_pixbuf_loader_write(loader, data, len, &err)) {
		g_object_unref(loader);
		char *message = err != NULL ? g_strdup(err->message) : g_strdup("unable to read png icon");
		if (err != NULL) {
			g_error_free(err);
		}
		return message;
	}

	if (!gdk_pixbuf_loader_close(loader, &err)) {
		g_object_unref(loader);
		char *message = err != NULL ? g_strdup(err->message) : g_strdup("unable to close png loader");
		if (err != NULL) {
			g_error_free(err);
		}
		return message;
	}

	GdkPixbuf *pixbuf = gdk_pixbuf_loader_get_pixbuf(loader);
	if (pixbuf == NULL) {
		g_object_unref(loader);
		return g_strdup("unable to decode png icon");
	}

	gtk_window_set_icon(window, pixbuf);
	g_object_unref(loader);
	return NULL;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func applyWindowIcon(window unsafe.Pointer, iconPNG []byte) error {
	if len(iconPNG) == 0 {
		return nil
	}

	data := C.CBytes(iconPNG)
	defer C.free(data)

	errMessage := C.dfw_set_window_icon((*C.GtkWindow)(window), (*C.uchar)(data), C.size_t(len(iconPNG)))
	if errMessage != nil {
		defer C.g_free(C.gpointer(errMessage))
		return fmt.Errorf("dfw: set window icon: %s", C.GoString(errMessage))
	}
	return nil
}
