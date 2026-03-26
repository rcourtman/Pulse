package cloudcp

import "net/http"

const controlPlaneFaviconSVG = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">
  <rect width="64" height="64" rx="14" fill="#0f172a"/>
  <path d="M18 18h13c9.941 0 18 8.059 18 18s-8.059 18-18 18H18V18zm11 10h-3v16h3c4.418 0 8-3.582 8-8s-3.582-8-8-8z" fill="#38bdf8"/>
  <circle cx="47" cy="18" r="5" fill="#1d4ed8"/>
</svg>
`

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
	_, _ = w.Write([]byte(controlPlaneFaviconSVG))
}

func handleControlPlaneFaviconICO(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.Redirect(w, r, "/favicon.svg", http.StatusMovedPermanently)
}
