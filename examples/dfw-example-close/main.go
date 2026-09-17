// dfw-example-close is a deliberately small fixture for the native close
// request lifecycle. it serves an inline page that shows whether a close
// request is pending and how many have been delivered, and offers explicit
// 'Keep open' and 'Close' controls while one is pending. nothing resolves
// automatically; the manual sequence in README.md is the proof.
package main

import (
	"image"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/michaelquigley/df/dd"
	"github.com/michaelquigley/df/dl"
	"github.com/michaelquigley/dfw/webview"
)

const (
	appID             = "com.quigley.dfw.example.close"
	appTitle          = "dfw Example Close"
	readHeaderTimeout = 5 * time.Second
)

type closeState struct {
	mu        sync.Mutex
	pending   *webview.CloseRequest
	delivered int
}

type statusResponse struct {
	State     string
	Delivered int
}

func main() {
	state := &closeState{}
	err := webview.Run(webview.App{
		AppID:          appID,
		Title:          appTitle,
		InitialSize:    image.Pt(640, 400),
		Listen:         state.listen,
		OnCloseRequest: state.onCloseRequest,
	})
	if err != nil {
		dl.Error(err)
		os.Exit(1)
	}
}

// onCloseRequest records the pending request; the page resolves it later.
func (s *closeState) onCloseRequest(request *webview.CloseRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = request
	s.delivered++
}

// take removes and returns the pending request so it can be resolved outside
// the mutex.
func (s *closeState) take() *webview.CloseRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	request := s.pending
	s.pending = nil
	return request
}

func (s *closeState) status() statusResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := "idle"
	if s.pending != nil {
		state = "pending"
	}
	return statusResponse{State: state, Delivered: s.delivered}
}

func (s *closeState) listen() (*http.Server, net.Listener, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/keep-open", s.handleKeepOpen)
	mux.HandleFunc("/api/close", s.handleClose)
	mux.HandleFunc("/", handleIndex)
	return &http.Server{Handler: mux, ReadHeaderTimeout: readHeaderTimeout}, listener, nil
}

func (s *closeState) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := dd.UnbindJSON(s.status())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func (s *closeState) handleKeepOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if request := s.take(); request != nil {
		request.KeepOpen()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *closeState) handleClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if request := s.take(); request != nil {
		request.Close()
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

const indexHTML = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>dfw Example Close</title>
<style>
  body { font-family: sans-serif; margin: 2rem; color: #222; }
  #state { font-weight: bold; }
  button { font-size: 1rem; padding: 0.5rem 1rem; margin-right: 0.5rem; }
  #controls[hidden] { display: none; }
</style>
</head>
<body>
<h1>dfw example close</h1>
<p>close request: <span id="state">idle</span></p>
<p>delivered: <span id="delivered">0</span></p>
<div id="controls" hidden>
  <button id="keep-open">Keep open</button>
  <button id="close">Close</button>
</div>
<p>use the native title-bar close control to create a request.</p>
<script>
  const state = document.getElementById('state');
  const delivered = document.getElementById('delivered');
  const controls = document.getElementById('controls');
  async function poll() {
    try {
      const response = await fetch('/api/status');
      const status = await response.json();
      state.textContent = status.state;
      delivered.textContent = status.delivered;
      controls.hidden = status.state !== 'pending';
    } catch (err) {
      state.textContent = 'unreachable';
    }
  }
  async function resolve(path) {
    await fetch(path, { method: 'POST' });
    await poll();
  }
  document.getElementById('keep-open').addEventListener('click', () => resolve('/api/keep-open'));
  document.getElementById('close').addEventListener('click', () => resolve('/api/close'));
  poll();
  setInterval(poll, 250);
</script>
</body>
</html>
`
