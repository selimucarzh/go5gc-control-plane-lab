package main

import (
	"embed"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"go5gc-control-plane-lab/internal/amf"
	"go5gc-control-plane-lab/internal/smf"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	amfAddr := ":8081"
	smfAddr := ":8082"
	uiAddr := ":8090"

	amfServer := amf.NewServer("amf-001")
	smfServer := smf.NewServer("smf-001", smf.NewHTTPAMFClient("http://127.0.0.1:8081", nil))

	startService("amf", amfAddr, amfServer.Handler())
	startService("smf", smfAddr, smfServer.Handler())

	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(staticFiles)))
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/api/amf/", proxy("http://127.0.0.1:8081", "/api/amf"))
	mux.HandleFunc("/api/smf/", proxy("http://127.0.0.1:8082", "/api/smf"))

	log.Printf("lab viewer listening on http://127.0.0.1%s", uiAddr)
	if err := http.ListenAndServe(uiAddr, mux); err != nil {
		log.Fatalf("run lab viewer: %v", err)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	content, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "index not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(content)
}

func startService(name, addr string, handler http.Handler) {
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 3 * time.Second,
	}

	go func() {
		log.Printf("%s listening on %s", name, addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("run %s: %v", name, err)
		}
	}()
}

func proxy(baseURL, stripPrefix string) http.HandlerFunc {
	client := &http.Client{Timeout: 4 * time.Second}

	return func(w http.ResponseWriter, r *http.Request) {
		targetPath := strings.TrimPrefix(r.URL.Path, stripPrefix)
		if targetPath == r.URL.Path || targetPath == "" {
			http.NotFound(w, r)
			return
		}

		target := baseURL + targetPath
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}

		req, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
		if err != nil {
			writeProxyError(w, http.StatusBadGateway, "build upstream request")
			return
		}
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))

		resp, err := client.Do(req)
		if err != nil {
			writeProxyError(w, http.StatusBadGateway, "upstream service is unavailable")
			return
		}
		defer resp.Body.Close()

		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}

func writeProxyError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + message + `"}`))
}
