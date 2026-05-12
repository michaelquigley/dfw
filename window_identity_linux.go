//go:build linux

package dfw

/*
#cgo pkg-config: gtk+-3.0

#include <gtk/gtk.h>
#include <gdk/gdk.h>
#include <stdlib.h>

static void dfw_prepare_window_identity(const char *app_id) {
	if (app_id == NULL || app_id[0] == '\0') {
		return;
	}

	g_set_prgname(app_id);
	gdk_set_program_class(app_id);
}

static void dfw_apply_window_identity(GtkWindow *window, const char *app_id) {
	if (window == NULL || app_id == NULL || app_id[0] == '\0') {
		return;
	}

	// webview_go returns an already-realized window, so WM_CLASS must come
	// from gdk_set_program_class() before the GTK window is created.
	gtk_window_set_icon_name(window, app_id);
}
*/
import "C"

import "unsafe"

func prepareNativeWindowIdentity(appID string) {
	if appID == "" {
		return
	}

	cAppID := C.CString(appID)
	defer C.free(unsafe.Pointer(cAppID))

	C.dfw_prepare_window_identity(cAppID)
}

func applyNativeWindowIdentity(window unsafe.Pointer, appID string) {
	if !validNativeWindow(window) || appID == "" {
		return
	}

	cAppID := C.CString(appID)
	defer C.free(unsafe.Pointer(cAppID))

	C.dfw_apply_window_identity((*C.GtkWindow)(window), cAppID)
}
