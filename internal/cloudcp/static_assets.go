package cloudcp

import (
	_ "embed"
	"net/http"
)

//go:embed favicon.svg
var controlPlaneFaviconSVG []byte

func handleControlPlaneFaviconSVG(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(controlPlaneFaviconSVG)
}

func handleControlPlaneFaviconICO(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.Redirect(w, r, "/favicon.svg", http.StatusMovedPermanently)
}
