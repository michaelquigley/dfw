//go:build linux

package dfw

/*
#cgo pkg-config: gtk+-3.0

#include <gtk/gtk.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
	GtkWindow *window;
	gulong configure_id;
	gulong destroy_id;
	int width;
	int height;
	int x;
	int y;
	int has_bounds;
	int has_location;
	int location_supported;
	int destroyed;
} dfw_window_bounds_tracker;

typedef struct {
	GtkWindow *window;
	int width;
	int height;
} dfw_window_size_request;

typedef struct {
	GtkWindow *window;
	int x;
	int y;
} dfw_window_location_request;

static int dfw_linux_location_supported() {
	GdkDisplay *display = gdk_display_get_default();
	if (display == NULL) {
		return 0;
	}
	const char *type_name = G_OBJECT_TYPE_NAME(display);
	if (type_name == NULL) {
		return 0;
	}
	return strstr(type_name, "X11") != NULL;
}

static void dfw_linux_update_window_bounds(dfw_window_bounds_tracker *tracker) {
	if (tracker == NULL || tracker->window == NULL) {
		return;
	}

	int width = 0;
	int height = 0;
	gtk_window_get_size(tracker->window, &width, &height);
	if (width > 0 && height > 0) {
		tracker->width = width;
		tracker->height = height;
		tracker->has_bounds = 1;
	}

	if (tracker->location_supported) {
		int x = 0;
		int y = 0;
		gtk_window_get_position(tracker->window, &x, &y);
		tracker->x = x;
		tracker->y = y;
		tracker->has_location = 1;
	} else {
		tracker->has_location = 0;
	}
}

static gboolean dfw_linux_configure_event(GtkWidget *widget, GdkEvent *event, gpointer data) {
	dfw_window_bounds_tracker *tracker = (dfw_window_bounds_tracker *)data;
	if (tracker == NULL) {
		return FALSE;
	}

	if (event != NULL && event->type == GDK_CONFIGURE) {
		GdkEventConfigure *configure = (GdkEventConfigure *)event;
		if (configure->width > 0 && configure->height > 0) {
			tracker->width = configure->width;
			tracker->height = configure->height;
			tracker->has_bounds = 1;
		}
		if (tracker->location_supported) {
			tracker->x = configure->x;
			tracker->y = configure->y;
			tracker->has_location = 1;
		}
	}

	return FALSE;
}

static void dfw_linux_destroy(GtkWidget *widget, gpointer data) {
	dfw_window_bounds_tracker *tracker = (dfw_window_bounds_tracker *)data;
	if (tracker == NULL) {
		return;
	}
	tracker->destroyed = 1;
	tracker->window = NULL;
}

static dfw_window_bounds_tracker *dfw_linux_start_window_bounds_tracker(GtkWindow *window) {
	if (window == NULL) {
		return NULL;
	}

	dfw_window_bounds_tracker *tracker = (dfw_window_bounds_tracker *)calloc(1, sizeof(dfw_window_bounds_tracker));
	if (tracker == NULL) {
		return NULL;
	}

	tracker->window = window;
	tracker->location_supported = dfw_linux_location_supported();
	tracker->configure_id = g_signal_connect(G_OBJECT(window), "configure-event", G_CALLBACK(dfw_linux_configure_event), tracker);
	tracker->destroy_id = g_signal_connect(G_OBJECT(window), "destroy", G_CALLBACK(dfw_linux_destroy), tracker);
	return tracker;
}

static void dfw_linux_window_bounds_snapshot(dfw_window_bounds_tracker *tracker, int *ok, int *width, int *height, int *x, int *y, int *has_location) {
	if (ok == NULL || width == NULL || height == NULL || x == NULL || y == NULL || has_location == NULL) {
		return;
	}

	*ok = 0;
	*width = 0;
	*height = 0;
	*x = 0;
	*y = 0;
	*has_location = 0;

	if (tracker == NULL) {
		return;
	}
	if (!tracker->destroyed) {
		dfw_linux_update_window_bounds(tracker);
	}
	if (!tracker->has_bounds || tracker->width <= 0 || tracker->height <= 0) {
		return;
	}

	*ok = 1;
	*width = tracker->width;
	*height = tracker->height;
	*x = tracker->x;
	*y = tracker->y;
	*has_location = tracker->has_location;
}

static void dfw_linux_window_bounds_tracker_free(dfw_window_bounds_tracker *tracker) {
	if (tracker == NULL) {
		return;
	}

	if (!tracker->destroyed && tracker->window != NULL) {
		if (tracker->configure_id != 0) {
			g_signal_handler_disconnect(G_OBJECT(tracker->window), tracker->configure_id);
		}
		if (tracker->destroy_id != 0) {
			g_signal_handler_disconnect(G_OBJECT(tracker->window), tracker->destroy_id);
		}
	}
	free(tracker);
}

static void dfw_linux_apply_window_size_now(GtkWindow *window, int width, int height) {
	if (window == NULL || width <= 0 || height <= 0) {
		return;
	}
	gtk_window_set_default_size(window, width, height);
	gtk_window_resize(window, width, height);
}

static gboolean dfw_linux_apply_window_size_idle(gpointer data) {
	dfw_window_size_request *request = (dfw_window_size_request *)data;
	if (request != NULL) {
		dfw_linux_apply_window_size_now(request->window, request->width, request->height);
	}
	return G_SOURCE_REMOVE;
}

static void dfw_linux_free_window_size_request(gpointer data) {
	dfw_window_size_request *request = (dfw_window_size_request *)data;
	if (request != NULL) {
		if (request->window != NULL) {
			g_object_unref(request->window);
		}
		free(request);
	}
}

static int dfw_linux_apply_window_size(GtkWindow *window, int width, int height) {
	if (window == NULL || width <= 0 || height <= 0) {
		return 0;
	}

	dfw_linux_apply_window_size_now(window, width, height);

	dfw_window_size_request *request = (dfw_window_size_request *)calloc(1, sizeof(dfw_window_size_request));
	if (request == NULL) {
		return 1;
	}
	request->window = GTK_WINDOW(g_object_ref(window));
	request->width = width;
	request->height = height;
	g_idle_add_full(G_PRIORITY_HIGH_IDLE, dfw_linux_apply_window_size_idle, request, dfw_linux_free_window_size_request);
	return 1;
}

static int dfw_linux_clamp(int value, int min, int max) {
	if (value < min) {
		return min;
	}
	if (value > max) {
		return max;
	}
	return value;
}

static void dfw_linux_apply_window_location_now(GtkWindow *window, int x, int y) {
	if (window == NULL || !dfw_linux_location_supported()) {
		return;
	}

	int width = 0;
	int height = 0;
	gtk_window_get_size(window, &width, &height);

	GdkDisplay *display = gtk_widget_get_display(GTK_WIDGET(window));
	if (display != NULL && width > 0 && height > 0) {
		GdkMonitor *monitor = gdk_display_get_monitor_at_point(display, x, y);
		if (monitor == NULL) {
			monitor = gdk_display_get_primary_monitor(display);
		}
		if (monitor != NULL) {
			GdkRectangle workarea;
			gdk_monitor_get_workarea(monitor, &workarea);
			int max_x = workarea.x + workarea.width - width;
			int max_y = workarea.y + workarea.height - height;
			if (max_x < workarea.x) {
				max_x = workarea.x;
			}
			if (max_y < workarea.y) {
				max_y = workarea.y;
			}
			x = dfw_linux_clamp(x, workarea.x, max_x);
			y = dfw_linux_clamp(y, workarea.y, max_y);
		}
	}

	gtk_window_move(window, x, y);
}

static gboolean dfw_linux_apply_window_location_idle(gpointer data) {
	dfw_window_location_request *request = (dfw_window_location_request *)data;
	if (request != NULL) {
		dfw_linux_apply_window_location_now(request->window, request->x, request->y);
	}
	return G_SOURCE_REMOVE;
}

static void dfw_linux_free_window_location_request(gpointer data) {
	dfw_window_location_request *request = (dfw_window_location_request *)data;
	if (request != NULL) {
		if (request->window != NULL) {
			g_object_unref(request->window);
		}
		free(request);
	}
}

static int dfw_linux_apply_window_location(GtkWindow *window, int x, int y) {
	if (window == NULL || !dfw_linux_location_supported()) {
		return 0;
	}

	dfw_linux_apply_window_location_now(window, x, y);

	dfw_window_location_request *request = (dfw_window_location_request *)calloc(1, sizeof(dfw_window_location_request));
	if (request == NULL) {
		return 1;
	}
	request->window = GTK_WINDOW(g_object_ref(window));
	request->x = x;
	request->y = y;
	g_idle_add_full(G_PRIORITY_HIGH_IDLE, dfw_linux_apply_window_location_idle, request, dfw_linux_free_window_location_request);
	return 1;
}
*/
import "C"

import (
	"image"
	"unsafe"
)

type linuxWindowBoundsTracker struct {
	tracker *C.dfw_window_bounds_tracker
}

func newNativeWindowBoundsTracker(window unsafe.Pointer) nativeWindowBoundsTracker {
	if !validNativeWindow(window) {
		return noopWindowBoundsTracker{}
	}
	tracker := C.dfw_linux_start_window_bounds_tracker((*C.GtkWindow)(window))
	if tracker == nil {
		return noopWindowBoundsTracker{}
	}
	return &linuxWindowBoundsTracker{tracker: tracker}
}

func (t *linuxWindowBoundsTracker) Bounds() (windowBounds, bool) {
	if t == nil || t.tracker == nil {
		return windowBounds{}, false
	}

	var ok C.int
	var width C.int
	var height C.int
	var x C.int
	var y C.int
	var hasLocation C.int
	C.dfw_linux_window_bounds_snapshot(t.tracker, &ok, &width, &height, &x, &y, &hasLocation)
	if ok == 0 {
		return windowBounds{}, false
	}
	return windowBounds{
		Width:       int(width),
		Height:      int(height),
		X:           int(x),
		Y:           int(y),
		HasLocation: hasLocation != 0,
	}, true
}

func (t *linuxWindowBoundsTracker) Close() {
	if t == nil || t.tracker == nil {
		return
	}
	C.dfw_linux_window_bounds_tracker_free(t.tracker)
	t.tracker = nil
}

func applyNativeWindowSize(window unsafe.Pointer, size image.Point) bool {
	if !validNativeWindow(window) || size.X <= 0 || size.Y <= 0 {
		return false
	}
	return C.dfw_linux_apply_window_size((*C.GtkWindow)(window), C.int(size.X), C.int(size.Y)) != 0
}

func applyNativeWindowLocation(window unsafe.Pointer, x int, y int) bool {
	if !validNativeWindow(window) {
		return false
	}
	return C.dfw_linux_apply_window_location((*C.GtkWindow)(window), C.int(x), C.int(y)) != 0
}
