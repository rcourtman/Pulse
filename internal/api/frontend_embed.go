package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

// Embed the entire frontend dist directory
//
//go:embed all:frontend-modern/dist
var embeddedFrontend embed.FS

// getFrontendFS returns the embedded frontend filesystem
func getFrontendFS() (http.FileSystem, error) {
	// Strip the prefix to serve files from root
	fsys, err := fs.Sub(embeddedFrontend, "frontend-modern/dist")
	if err != nil {
		return nil, err
	}
	return http.FS(fsys), nil
}

// serveFrontendHandler returns a handler for serving the embedded frontend
func serveFrontendHandler() http.HandlerFunc {
	// Get the embedded filesystem
	fsys, err := getFrontendFS()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get embedded frontend")
	}

	fileServer := http.FileServer(fsys)

	return func(w http.ResponseWriter, r *http.Request) {
		// Clean the path
		p := r.URL.Path

		// Default to index.html for root
		if p == "/" {
			p = "/index.html"
		}

		// Remove leading slash for filesystem lookup
		lookupPath := strings.TrimPrefix(p, "/")

		// Check if file exists in embedded FS
		file, err := fsys.Open(lookupPath)
		if err == nil {
			file.Close()
			// File exists, serve it
			fileServer.ServeHTTP(w, r)
			return
		}

		// For SPA routing, serve index.html for non-API routes
		if !strings.HasPrefix(p, "/api/") &&
			!strings.HasPrefix(p, "/ws") &&
			!strings.HasPrefix(p, "/socket.io/") {
			// Serve index.html for client-side routing
			r.URL.Path = "/index.html"
			fileServer.ServeHTTP(w, r)
			return
		}

		// Not found
		http.NotFound(w, r)
	}
}
