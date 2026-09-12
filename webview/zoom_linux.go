//go:build linux

package webview

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

typedef struct {
	GtkWidget *window;
	WebKitWebView *view;
	gulong key_handler;
	gulong destroy_handler;
	int percent;
} dfw_zoom;

static gboolean dfw_zoom_key(GtkWidget *widget, GdkEventKey *event, gpointer data) {
	if (!(event->state & GDK_CONTROL_MASK) ||
		(event->state & (GDK_MOD1_MASK | GDK_SUPER_MASK | GDK_META_MASK))) return FALSE;
	dfw_zoom *zoom = data;
	int next = zoom->percent;
	switch (event->keyval) {
	case GDK_KEY_plus: case GDK_KEY_equal: case GDK_KEY_KP_Add: next += 10; break;
	case GDK_KEY_minus: case GDK_KEY_KP_Subtract: next -= 10; break;
	case GDK_KEY_0: case GDK_KEY_KP_0: next = 100; break;
	default: return FALSE;
	}
	zoom->percent = CLAMP(next, 50, 200);
	webkit_web_view_set_zoom_level(zoom->view, zoom->percent / 100.0);
	return TRUE;
}

static void dfw_zoom_destroy(GtkWidget *widget, gpointer data) {
	dfw_zoom *zoom = data;
	zoom->window = NULL;
	zoom->view = NULL;
}

static WebKitWebView *dfw_zoom_find_view(GtkWidget *widget) {
	if (WEBKIT_IS_WEB_VIEW(widget)) return WEBKIT_WEB_VIEW(widget);
	if (!GTK_IS_CONTAINER(widget)) return NULL;
	GList *children = gtk_container_get_children(GTK_CONTAINER(widget));
	WebKitWebView *view = NULL;
	for (GList *item = children; item && !view; item = item->next)
		view = dfw_zoom_find_view(GTK_WIDGET(item->data));
	g_list_free(children);
	return view;
}

static dfw_zoom *dfw_zoom_start(GtkWidget *window, int percent) {
	WebKitWebView *view = dfw_zoom_find_view(window);
	if (!view) return NULL;
	dfw_zoom *zoom = g_new0(dfw_zoom, 1);
	zoom->percent = percent;
	zoom->window = window;
	zoom->view = view;
	webkit_web_view_set_zoom_level(view, percent / 100.0);
	// run before the window's default handler forwards keys to the editor.
	zoom->key_handler = g_signal_connect(window, "key-press-event", G_CALLBACK(dfw_zoom_key), zoom);
	zoom->destroy_handler = g_signal_connect(window, "destroy", G_CALLBACK(dfw_zoom_destroy), zoom);
	return zoom;
}

static void dfw_zoom_stop(dfw_zoom *zoom) {
	if (zoom->window) {
		g_signal_handler_disconnect(zoom->window, zoom->key_handler);
		g_signal_handler_disconnect(zoom->window, zoom->destroy_handler);
	}
	g_free(zoom);
}
*/
import "C"

import "unsafe"

type linuxZoomController struct{ zoom *C.dfw_zoom }

func newNativeZoom(window unsafe.Pointer, percent int) nativeZoomController {
	if !validNativeWindow(window) {
		return nil
	}
	zoom := C.dfw_zoom_start((*C.GtkWidget)(window), C.int(percent))
	if zoom == nil {
		return nil
	}
	return &linuxZoomController{zoom: zoom}
}

func (z *linuxZoomController) Percent() int { return int(z.zoom.percent) }
func (z *linuxZoomController) Close() {
	C.dfw_zoom_stop(z.zoom)
	z.zoom = nil
}
