// dfw-example-pdf is a native acceptance fixture, not a product save implementation.
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"image"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/michaelquigley/df/dd"
	"github.com/michaelquigley/dfw/webview"
)

func main() {
	chromium := flag.String("chromium", "", "system Chromium executable")
	delay := flag.Duration("delay", 150*time.Millisecond, "delay printable resources to exercise cancellation")
	flag.Parse()
	exporter := webview.NewPDFExporter(webview.PDFOptions{Executable: *chromium})
	token := rand.Text()
	var exportMu sync.Mutex
	listen := func() (*http.Server, net.Listener, error) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, nil, err
		}
		origin := "http://" + listener.Addr().String()
		mux := http.NewServeMux()
		mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(strings.ReplaceAll(page, "TOKEN", token)))
		})
		mux.HandleFunc("GET /document", func(w http.ResponseWriter, r *http.Request) {
			if !wait(r.Context(), *delay) {
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			body := document + strings.Repeat("<tr><td>row</td><td>a complete row of printable text</td></tr>", 90) + "</tbody></table><p>final paragraph marker</p>"
			if r.URL.Query().Get("broken") == "1" {
				body += `<img src="/missing.svg">`
			}
			_, _ = w.Write([]byte(body))
		})
		mux.HandleFunc("GET /image.svg", func(w http.ResponseWriter, r *http.Request) {
			if !wait(r.Context(), *delay) {
				return
			}
			w.Header().Set("Content-Type", "image/svg+xml")
			_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="200" height="50"><rect width="200" height="50" fill="#9a5b2b"/></svg>`))
		})
		mux.HandleFunc("POST /export", func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if !exportMu.TryLock() {
				http.Error(w, "export already active", http.StatusConflict)
				return
			}
			defer exportMu.Unlock()
			ctx, cancel := context.WithCancel(r.Context())
			defer cancel()
			stop := context.AfterFunc(exporter.WindowContext(), cancel)
			defer stop()
			home, _ := os.UserHomeDir()
			choice, err := exporter.Choose(ctx, webview.PDFDestinationRequest{SuggestedName: "dfw-proof.pdf", InitialDirectory: home})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if choice.Cancelled {
				respond(w, "cancelled")
				return
			}
			url := origin + "/document"
			if r.URL.Query().Get("broken") == "1" {
				url += "?broken=1"
			}
			data, err := exporter.Render(ctx, webview.PDFDocument{URL: url, Resources: []string{origin + "/image.svg", origin + "/missing.svg"}})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if ctx.Err() != nil || exporter.WindowContext().Err() != nil {
				http.Error(w, "export cancelled", http.StatusConflict)
				return
			}
			// the fixture has no piece or sidecar to protect. real products apply
			// their own expected-file checks before entering this publication step.
			if err := publish(choice.Path, data); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			respond(w, fmt.Sprintf("saved '%s'", choice.Path))
		})
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Host != listener.Addr().String() || (r.Method == http.MethodPost && r.Header.Get("Origin") != "" && r.Header.Get("Origin") != origin) {
				http.Error(w, "wrong origin", http.StatusForbidden)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			mux.ServeHTTP(w, r)
		})
		return &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}, listener, nil
	}
	if err := webview.Run(webview.App{AppID: "com.quigley.dfw.pdf-example", Title: "dfw PDF fixture", InitialSize: image.Pt(700, 350), Listen: listen, PDF: exporter}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
func respond(w http.ResponseWriter, message string) {
	data, err := dd.UnbindJSON(struct{ Message string }{message})
	if err != nil {
		http.Error(w, "encode result", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}
func publish(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".dfw-pdf-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

const page = `<!doctype html><meta charset="utf-8"><title>dfw PDF fixture</title>
<style>body{font:18px/1.5 serif;margin:2rem}button{font:inherit;margin-right:1rem}#status{white-space:pre-wrap}</style>
<h1>dfw PDF fixture</h1><p>choose a destination once; the native chooser handles overwrites.</p>
<button id="export">save as PDF</button><button id="broken">try broken image</button><button id="cancel">cancel export</button><p id="status">ready</p>
<script>
let controller;
async function run(broken){
 if(controller)return;
 controller=new AbortController();document.querySelector('#status').textContent='working';
 try{const r=await fetch('/export'+(broken?'?broken=1':''),{method:'POST',headers:{Authorization:'Bearer TOKEN'},signal:controller.signal});
 const text=r.ok?(await r.json()).message:await r.text();document.querySelector('#status').textContent=text;
 }catch(e){document.querySelector('#status').textContent='interrupted; if publication had begun, check the destination';}
 finally{controller=null;}
}
document.querySelector('#export').onclick=()=>run(false);
document.querySelector('#broken').onclick=()=>run(true);
document.querySelector('#cancel').onclick=()=>controller?.abort();
</script>`

const document = `<!doctype html><html><head><meta charset="utf-8"><link rel="icon" href="data:,"><title>dfw PDF proof</title>
<style>@page{size:letter;margin:18mm;@bottom-center{content:"page " counter(page) " of " counter(pages);font-size:9pt}}
body{font:12pt/1.5 serif;color:#1f1d1a}a{color:#9a5b2b}table{border-collapse:collapse;width:100%}th,td{border:1px solid #ddd;padding:6pt}thead{display:table-header-group}tr{break-inside:avoid}</style>
</head><body><h1>dfw PDF proof</h1><p>selectable text, with <em>italics</em> and <strong>bold</strong>.</p><a href="https://example.com/pdf-proof">clickable link</a><p><img src="/image.svg"></p>
<table><thead><tr><th>repeated heading</th><th>description</th></tr></thead><tbody>`
