#ifndef DFW_PDF_DIALOG_H
#define DFW_PDF_DIALOG_H
#include <gtk/gtk.h>
#include <stdint.h>
typedef struct dfw_pdf_dialog dfw_pdf_dialog;
dfw_pdf_dialog *dfw_pdf_dialog_new(GtkWindow *parent, const char *name, const char *directory, uintptr_t handle);
void dfw_pdf_dialog_show(dfw_pdf_dialog *dialog);
void dfw_pdf_dialog_free(dfw_pdf_dialog *dialog);
#endif
