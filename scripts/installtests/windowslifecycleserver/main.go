package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:17655", "HTTP listen address")
	agentBinary := flag.String("agent-binary", "", "Windows agent binary to serve")
	version := flag.String("version", "", "Version returned by /api/version")
	flag.Parse()

	if strings.TrimSpace(*agentBinary) == "" || strings.TrimSpace(*version) == "" {
		log.Fatal("--agent-binary and --version are required")
	}

	binaryPath, err := filepath.Abs(*agentBinary)
	if err != nil {
		log.Fatalf("resolve agent binary: %v", err)
	}
	binary, err := os.ReadFile(binaryPath)
	if err != nil {
		log.Fatalf("read agent binary: %v", err)
	}
	digest := sha256.Sum256(binary)
	checksum := hex.EncodeToString(digest[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version":                  *version,
			"agentUpdateTargetVersion": *version,
			"channel":                  "stable",
		})
	})
	mux.HandleFunc("/download/pulse-agent", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("arch") != "windows-amd64" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(binary)))
		w.Header().Set("X-Checksum-Sha256", checksum)
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write(binary)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	})

	server := &http.Server{
		Addr:              *listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       30 * time.Second,
	}
	log.Printf("Windows lifecycle server listening on %s with %s (%s)", *listen, filepath.Base(binaryPath), *version)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
