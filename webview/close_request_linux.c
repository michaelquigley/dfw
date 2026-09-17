#include "close_request_linux.h"

#include <stdlib.h>

#include "_cgo_export.h"

struct dfw_close_interceptor {
	GtkWindow *window;
	gulong delete_id;
	gulong destroy_id;
	uintptr_t handle;
};

static gboolean dfw_close_delete_event(GtkWidget *widget, GdkEvent *event, gpointer data) {
	dfw_close_interceptor *interceptor = (dfw_close_interceptor *)data;
	if (interceptor == NULL || interceptor->handle == 0) {
		return FALSE;
	}
	// TRUE stops the signal, so GTK does not destroy the window; FALSE lets the
	// default handler run the ordinary close.
	return dfwLinuxCloseRequested(interceptor->handle) ? TRUE : FALSE;
}

static void dfw_close_destroy(GtkWidget *widget, gpointer data) {
	dfw_close_interceptor *interceptor = (dfw_close_interceptor *)data;
	if (interceptor == NULL) {
		return;
	}
	interceptor->window = NULL;
}

dfw_close_interceptor *dfw_close_interceptor_install(GtkWindow *window, uintptr_t handle) {
	if (window == NULL || handle == 0) {
		return NULL;
	}

	dfw_close_interceptor *interceptor = (dfw_close_interceptor *)calloc(1, sizeof(dfw_close_interceptor));
	if (interceptor == NULL) {
		return NULL;
	}
	interceptor->window = window;
	interceptor->handle = handle;

	interceptor->delete_id = g_signal_connect(G_OBJECT(window), "delete-event", G_CALLBACK(dfw_close_delete_event), interceptor);
	if (interceptor->delete_id == 0) {
		free(interceptor);
		return NULL;
	}
	interceptor->destroy_id = g_signal_connect(G_OBJECT(window), "destroy", G_CALLBACK(dfw_close_destroy), interceptor);
	if (interceptor->destroy_id == 0) {
		g_signal_handler_disconnect(G_OBJECT(window), interceptor->delete_id);
		free(interceptor);
		return NULL;
	}
	return interceptor;
}

void dfw_close_interceptor_free(dfw_close_interceptor *interceptor) {
	if (interceptor == NULL) {
		return;
	}
	if (interceptor->window != NULL) {
		if (interceptor->delete_id != 0) {
			g_signal_handler_disconnect(G_OBJECT(interceptor->window), interceptor->delete_id);
		}
		if (interceptor->destroy_id != 0) {
			g_signal_handler_disconnect(G_OBJECT(interceptor->window), interceptor->destroy_id);
		}
	}
	free(interceptor);
}
