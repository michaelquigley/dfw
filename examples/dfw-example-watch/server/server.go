package server

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/michaelquigley/df/dd"
	"github.com/michaelquigley/dfw/examples/dfw-example-watch/watcher"
)

const readHeaderTimeout = 5 * time.Second

type Server struct {
	assets fs.FS
	watch  *watcher.Watcher
}

type StatusResponse struct {
	App            string
	AppID          string
	Root           string
	StartedAt      time.Time
	EventCount     int64
	BufferCapacity int
}

type EventsResponse struct {
	Events []watcher.EventRecord
}

type ErrorResponse struct {
	Error string
}

// Listen returns a dfw Listen callback for the example server.
func Listen(assets fs.FS, watch *watcher.Watcher) func() (*http.Server, net.Listener, error) {
	return func() (*http.Server, net.Listener, error) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, nil, err
		}

		return &http.Server{
			Handler:           NewHandler(assets, watch),
			ReadHeaderTimeout: readHeaderTimeout,
		}, listener, nil
	}
}

// NewHandler builds the HTTP handler used by run and daemon modes.
func NewHandler(assets fs.FS, watch *watcher.Watcher) http.Handler {
	s := &Server{
		assets: assets,
		watch:  watch,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/", s.handleStatic)
	return mux
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	snapshot := s.watch.Snapshot()
	writeJSON(w, http.StatusOK, StatusResponse{
		App:            "dfw Example Watch",
		AppID:          "com.quigley.dfw.example.watch",
		Root:           snapshot.Root,
		StartedAt:      snapshot.StartedAt,
		EventCount:     snapshot.EventCount,
		BufferCapacity: snapshot.BufferCapacity,
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, EventsResponse{
		Events: s.watch.Events(),
	})
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	name := cleanAssetPath(r.URL.Path)
	if name == "" {
		s.serveAsset(w, r, "index.html")
		return
	}

	if ok, err := s.assetExists(name); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("read web bundle: %v", err))
		return
	} else if ok {
		s.serveAsset(w, r, name)
		return
	}

	s.serveAsset(w, r, "index.html")
}

func (s *Server) serveAsset(w http.ResponseWriter, r *http.Request, name string) {
	data, err := fs.ReadFile(s.assets, name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("read web bundle: %v", err))
		return
	}

	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, path.Base(name), time.Time{}, bytes.NewReader(data))
}

func (s *Server) assetExists(name string) (bool, error) {
	file, err := s.assets.Open(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
}

func cleanAssetPath(requestPath string) string {
	cleaned := strings.TrimPrefix(path.Clean("/"+requestPath), "/")
	if cleaned == "." || cleaned == "/" {
		return ""
	}
	return cleaned
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	data, err := dd.UnbindJSON(value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("encode response: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	data, err := dd.UnbindJSON(ErrorResponse{Error: message})
	if err != nil {
		http.Error(w, message, status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
