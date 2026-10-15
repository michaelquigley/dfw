//go:build linux

package webview

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>

typedef struct {
	GtkWidget *window;
	gulong key_handler;
	gulong state_handler;
	gulong destroy_handler;
	int fullscreen;
} dfw_fullscreen;

static gboolean dfw_fullscreen_key(GtkWidget *widget, GdkEventKey *event, gpointer data) {
	if (event->keyval != GDK_KEY_F11 ||
		(event->state & (GDK_CONTROL_MASK | GDK_SHIFT_MASK | GDK_MOD1_MASK | GDK_SUPER_MASK | GDK_META_MASK))) return FALSE;
	dfw_fullscreen *fullscreen = data;
	if (fullscreen->fullscreen) gtk_window_unfullscreen(GTK_WINDOW(widget));
	else gtk_window_fullscreen(GTK_WINDOW(widget));
	return TRUE;
}

// the state is what the window manager reports, not what was last asked for:
// a request can be refused, and a window manager shortcut can change it.
static gboolean dfw_fullscreen_state(GtkWidget *widget, GdkEventWindowState *event, gpointer data) {
	dfw_fullscreen *fullscreen = data;
	fullscreen->fullscreen = (event->new_window_state & GDK_WINDOW_STATE_FULLSCREEN) != 0;
	return FALSE;
}

static void dfw_fullscreen_destroy(GtkWidget *widget, gpointer data) {
	dfw_fullscreen *fullscreen = data;
	fullscreen->window = NULL;
}

static dfw_fullscreen *dfw_fullscreen_start(GtkWidget *window, int initial) {
	if (!GTK_IS_WINDOW(window)) return NULL;
	dfw_fullscreen *fullscreen = g_new0(dfw_fullscreen, 1);
	fullscreen->window = window;
	// run before the window's default handler forwards keys to the editor.
	fullscreen->key_handler = g_signal_connect(window, "key-press-event", G_CALLBACK(dfw_fullscreen_key), fullscreen);
	fullscreen->state_handler = g_signal_connect(window, "window-state-event", G_CALLBACK(dfw_fullscreen_state), fullscreen);
	fullscreen->destroy_handler = g_signal_connect(window, "destroy", G_CALLBACK(dfw_fullscreen_destroy), fullscreen);
	if (initial) gtk_window_fullscreen(GTK_WINDOW(window));
	return fullscreen;
}

static void dfw_fullscreen_stop(dfw_fullscreen *fullscreen) {
	if (fullscreen->window) {
		g_signal_handler_disconnect(fullscreen->window, fullscreen->key_handler);
		g_signal_handler_disconnect(fullscreen->window, fullscreen->state_handler);
		g_signal_handler_disconnect(fullscreen->window, fullscreen->destroy_handler);
	}
	g_free(fullscreen);
}
*/
import "C"

import "unsafe"

type linuxFullscreenController struct{ fullscreen *C.dfw_fullscreen }

// newNativeFullscreen binds F11 to toggle the window's fullscreen state and
// follows that state from the window manager. initial asks for fullscreen at
// once; the window size and location already applied stay the windowed ones.
func newNativeFullscreen(window unsafe.Pointer, initial bool) nativeFullscreenController {
	if !validNativeWindow(window) {
		return nil
	}
	start := C.int(0)
	if initial {
		start = 1
	}
	fullscreen := C.dfw_fullscreen_start((*C.GtkWidget)(window), start)
	if fullscreen == nil {
		return nil
	}
	return &linuxFullscreenController{fullscreen: fullscreen}
}

func (f *linuxFullscreenController) Fullscreen() bool { return f.fullscreen.fullscreen != 0 }
func (f *linuxFullscreenController) Close() {
	C.dfw_fullscreen_stop(f.fullscreen)
	f.fullscreen = nil
}
