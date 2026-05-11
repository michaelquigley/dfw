package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/michaelquigley/df/dd"
	"github.com/michaelquigley/dfw/examples/dfw-example-watch/watcher"
	"github.com/stretchr/testify/require"
)

func TestAPIStatus(t *testing.T) {
	watch, err := watcher.New(t.TempDir())
	require.NoError(t, err)
	defer watch.Close()

	handler := NewHandler(testAssets(), watch)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/status", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "event_count")

	status := StatusResponse{}
	require.NoError(t, dd.BindJSON(&status, recorder.Body.Bytes()))
	require.Equal(t, "dfw Example Watch", status.App)
	require.Equal(t, watch.Root(), status.Root)
}

func TestStaticFallbackServesIndex(t *testing.T) {
	handler := NewHandler(testAssets(), noopWatcher(t))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/missing/route", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	body, err := io.ReadAll(recorder.Result().Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "example shell")
}

func testAssets() fstest.MapFS {
	return fstest.MapFS{
		"index.html": {
			Data: []byte("<!doctype html><title>example shell</title>"),
		},
	}
}

func noopWatcher(t *testing.T) *watcher.Watcher {
	t.Helper()

	watch, err := watcher.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = watch.Close()
	})
	return watch
}
