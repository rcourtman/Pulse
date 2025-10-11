package api

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// Embed the entire frontend dist directory
//
//go:embed all:frontend-modern/dist
var embeddedFrontend embed.FS

var (
	devProxyOnce sync.Once
	devProxy     *httputil.ReverseProxy
	devProxyErr  error
	devProxyURL  string
)

func getFrontendDevProxy() (*httputil.ReverseProxy, error) {
	devProxyOnce.Do(func() {
		devURL := strings.TrimSpace(os.Getenv("FRONTEND_DEV_SERVER"))
		if devURL == "" {
			return
		}

		target, err := url.Parse(devURL)
		if err != nil {
			devProxyErr = err
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Error().Err(err).Str("path", r.URL.Path).Msg("Frontend dev proxy error")
			w.WriteHeader(http.StatusBadGateway)
		}
		devProxy = proxy
		devProxyURL = target.String()
		log.Warn().Str("frontend_dev_server", devProxyURL).Msg("Serving frontend via development proxy")
	})

	if devProxyErr != nil {
		return nil, devProxyErr
	}
	return devProxy, nil
}

// getFrontendFS returns the embedded frontend filesystem
func getFrontendFS() (http.FileSystem, error) {
	if dir := strings.TrimSpace(os.Getenv("PULSE_FRONTEND_DIR")); dir != "" {
		log.Warn().Str("frontend_dir", dir).Msg("Serving frontend from filesystem override")
		return http.Dir(dir), nil
	}

	// Strip the prefix to serve files from root
	fsys, err := fs.Sub(embeddedFrontend, "frontend-modern/dist")
	if err != nil {
		return nil, err
	}
	return http.FS(fsys), nil
}

// serveFrontendHandler returns a handler for serving the embedded frontend
func serveFrontendHandler() http.HandlerFunc {
	if proxy, err := getFrontendDevProxy(); err != nil {
		log.Error().Err(err).Msg("Failed to initialize frontend dev proxy, falling back to embedded assets")
	} else if proxy != nil {
		return func(w http.ResponseWriter, r *http.Request) {
			proxy.ServeHTTP(w, r)
		}
	}

	// Get the embedded filesystem
	fsys, err := getFrontendFS()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get embedded frontend")
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Clean the path
		p := r.URL.Path

		// Handle root path specially to avoid FileServer's directory redirect
		// Issue #334: Serve index.html directly without using FileServer for root
		if p == "/" || p == "" {
			// Directly serve index.html content
			file, err := fsys.Open("index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer file.Close()

			// Check that it's not a directory
			_, err = file.Stat()
			if err != nil {
				http.NotFound(w, r)
				return
			}

			// Read the file content
			content, err := io.ReadAll(file)
			if err != nil {
				http.NotFound(w, r)
				return
			}

			// Serve the content with cache-busting headers
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			w.Write(content)
			return
		}

		// Remove leading slash for filesystem lookup
		lookupPath := strings.TrimPrefix(p, "/")

		// Check if file exists in embedded FS
		file, err := fsys.Open(lookupPath)
		if err == nil {
			defer file.Close()

			// Get file info
			stat, err := file.Stat()
			if err == nil && !stat.IsDir() {
				// Read and serve the file
				content, err := io.ReadAll(file)
				if err == nil {
					// Detect content type
					contentType := "application/octet-stream"
					if strings.HasSuffix(lookupPath, ".html") {
						contentType = "text/html; charset=utf-8"
					} else if strings.HasSuffix(lookupPath, ".css") {
						contentType = "text/css; charset=utf-8"
					} else if strings.HasSuffix(lookupPath, ".js") {
						contentType = "application/javascript; charset=utf-8"
						// Add cache-busting headers for JS files to force reload
						w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
						w.Header().Set("Pragma", "no-cache")
						w.Header().Set("Expires", "0")
					} else if strings.HasSuffix(lookupPath, ".json") {
						contentType = "application/json"
					} else if strings.HasSuffix(lookupPath, ".svg") {
						contentType = "image/svg+xml"
					}

					w.Header().Set("Content-Type", contentType)
					w.Write(content)
					return
				}
			}
		}

		// For SPA routing, serve index.html for non-API routes
		if !strings.HasPrefix(p, "/api/") &&
			!strings.HasPrefix(p, "/ws") &&
			!strings.HasPrefix(p, "/socket.io/") {
			// Serve index.html for client-side routing
			indexFile, err := fsys.Open("index.html")
			if err == nil {
				defer indexFile.Close()
				content, err := io.ReadAll(indexFile)
				if err == nil {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Set("Pragma", "no-cache")
					w.Header().Set("Expires", "0")
					w.Write(content)
					return
				}
			}
		}

		// Not found
		http.NotFound(w, r)
	}
}
