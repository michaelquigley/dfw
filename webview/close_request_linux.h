#ifndef DFW_CLOSE_REQUEST_LINUX_H
#define DFW_CLOSE_REQUEST_LINUX_H

#include <gtk/gtk.h>
#include <stdint.h>

typedef struct dfw_close_interceptor dfw_close_interceptor;

// dfw_close_interceptor_install connects a delete-event handler that asks Go,
// through the cgo handle, whether the native close must be consumed. it
// returns NULL when the handlers cannot be installed.
dfw_close_interceptor *dfw_close_interceptor_install(GtkWindow *window, uintptr_t handle);

// dfw_close_interceptor_free disconnects the handlers and releases the
// interceptor. the caller deletes the cgo handle afterwards.
void dfw_close_interceptor_free(dfw_close_interceptor *interceptor);

#endif
