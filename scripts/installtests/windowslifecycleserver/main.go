package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:17655", "HTTP listen address")
	agentBinary := flag.String("agent-binary", "", "Windows agent binary to serve")
	version := flag.String("version", "", "Version returned by /api/version")
	uninstallMode := flag.String("uninstall-mode", "confirm", "Fixed synthetic uninstall response")
	flag.Parse()

	if err := validateListen(*listen); err != nil {
		log.Fatal(err)
	}
	uninstall, err := newUninstallProof(*uninstallMode)
	if err != nil {
		log.Fatal(err)
	}
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
	mux.HandleFunc("/api/agents/agent/uninstall", uninstall.confirm)
	mux.HandleFunc("/__pulse_lifecycle__/uninstall", uninstall.observe)
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

// Public synthetic data, not a project credential. This peer is loopback-only
// and accepts exactly the identity and saved token installed by the harness.
const lifecycleAgentID = "windows-lifecycle-agent"
const lifecycleToken = "windows-lifecycle-fixture-not-a-secret"
const privateResponseMarker = "lifecycle-private-response-must-not-appear"

func validateListen(address string) error {
	host, _, err := net.SplitHostPort(address)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("lifecycle peer requires a literal loopback listen address")
	}
	return nil
}

type uninstallReceipt struct {
	Mode          string `json:"mode"`
	Requests      int    `json:"requests"`
	Authenticated int    `json:"authenticated"`
	ExactIdentity int    `json:"exactIdentity"`
}

type uninstallProof struct {
	mu      sync.Mutex
	receipt uninstallReceipt
}

func newUninstallProof(mode string) (*uninstallProof, error) {
	switch mode {
	case "confirm", "http-503", "wrong-id", "string-success", "missing-id":
		return &uninstallProof{receipt: uninstallReceipt{Mode: mode}}, nil
	default:
		return nil, fmt.Errorf("unknown lifecycle uninstall mode")
	}
}

// Count actual requests, not installer success messages. The observation never
// returns a token, request body, hostname, binary or private response content.
func (p *uninstallProof) observe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(p.receipt)
}

func (p *uninstallProof) confirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		AgentID string `json:"agentId"`
	}
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&request)
	authenticated := r.Header.Get("X-API-Token") == lifecycleToken
	exact := err == nil && request.AgentID == lifecycleAgentID
	p.mu.Lock()
	p.receipt.Requests++
	if authenticated {
		p.receipt.Authenticated++
	}
	if exact {
		p.receipt.ExactIdentity++
	}
	mode := p.receipt.Mode
	p.mu.Unlock()
	if !authenticated {
		http.Error(w, privateResponseMarker, http.StatusUnauthorized)
		return
	}
	if !exact {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	response := map[string]any{"success": true, "agentId": request.AgentID}
	switch mode {
	case "http-503":
		http.Error(w, privateResponseMarker, http.StatusServiceUnavailable)
		return
	case "wrong-id":
		response["agentId"] = "same-name-other-host"
	case "string-success":
		response["success"] = "true"
	case "missing-id":
		delete(response, "agentId")
	}
	_ = json.NewEncoder(w).Encode(response)
}
