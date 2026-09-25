#include "pdf_dialog_linux.h"
#include "_cgo_export.h"

struct dfw_pdf_dialog {
    GtkFileChooserNative *chooser;
    gulong response;
    uintptr_t handle;
};

static void dfw_pdf_response(GtkNativeDialog *native, gint response, gpointer data) {
    dfw_pdf_dialog *dialog = data;
    char *path = NULL;
    if (response == GTK_RESPONSE_ACCEPT)
        path = gtk_file_chooser_get_filename(GTK_FILE_CHOOSER(dialog->chooser));
    // the Go callback may disconnect and free dialog. retain nothing across it.
    uintptr_t handle = dialog->handle;
    dfwPDFChosen(handle, response == GTK_RESPONSE_ACCEPT, path);
    g_free(path);
}

dfw_pdf_dialog *dfw_pdf_dialog_new(GtkWindow *parent, const char *name, const char *directory, uintptr_t handle) {
    if (!parent || !handle) return NULL;
    dfw_pdf_dialog *dialog = g_new0(dfw_pdf_dialog, 1);
    dialog->chooser = gtk_file_chooser_native_new("save as PDF", parent, GTK_FILE_CHOOSER_ACTION_SAVE, "save", "cancel");
    dialog->handle = handle;
    GtkFileChooser *chooser = GTK_FILE_CHOOSER(dialog->chooser);
    gtk_native_dialog_set_modal(GTK_NATIVE_DIALOG(dialog->chooser), TRUE);
    gtk_file_chooser_set_local_only(chooser, TRUE);
    gtk_file_chooser_set_do_overwrite_confirmation(chooser, TRUE);
    gtk_file_chooser_set_current_name(chooser, name);
    if (directory && *directory) gtk_file_chooser_set_current_folder(chooser, directory);
    GtkFileFilter *filter = gtk_file_filter_new();
    gtk_file_filter_set_name(filter, "PDF documents");
    gtk_file_filter_add_mime_type(filter, "application/pdf");
    gtk_file_filter_add_pattern(filter, "*.pdf");
    gtk_file_filter_add_pattern(filter, "*.PDF");
    gtk_file_chooser_add_filter(chooser, filter);
    dialog->response = g_signal_connect(dialog->chooser, "response", G_CALLBACK(dfw_pdf_response), dialog);
    return dialog;
}

void dfw_pdf_dialog_show(dfw_pdf_dialog *dialog) {
    gtk_native_dialog_show(GTK_NATIVE_DIALOG(dialog->chooser));
}

void dfw_pdf_dialog_free(dfw_pdf_dialog *dialog) {
    if (!dialog) return;
    g_signal_handler_disconnect(dialog->chooser, dialog->response);
    gtk_native_dialog_destroy(GTK_NATIVE_DIALOG(dialog->chooser));
    g_object_unref(dialog->chooser);
    g_free(dialog);
}
