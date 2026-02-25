package api

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// cspNoncePlaceholder is replaced at serve time with the per-request nonce.
var cspNoncePlaceholder = []byte("__CSP_NONCE__")

// serveIndexWithNonce writes index.html content to w, replacing any
// __CSP_NONCE__ placeholders with the nonce from the request context.
func serveIndexWithNonce(w http.ResponseWriter, r *http.Request, content []byte) {
	if nonce := CSPNonceFromContext(r.Context()); nonce != "" {
		content = bytes.ReplaceAll(content, cspNoncePlaceholder, []byte(nonce))
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(content)
}

// Embed the entire frontend dist directory
//
//go:embed all:frontend-modern/dist
var embeddedFrontend embed.FS

var (
	devProxyOnce sync.Once
	devProxy     *httputil.ReverseProxy
	devProxyErr  error
)

func getFrontendDevProxy() (*httputil.ReverseProxy, error) {
	devProxyOnce.Do(func() {
		devURL := utils.GetenvTrim("FRONTEND_DEV_SERVER")
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
		log.Warn().Str("frontend_dev_server", target.String()).Msg("Serving frontend via development proxy")
	})

	if devProxyErr != nil {
		return nil, devProxyErr
	}
	return devProxy, nil
}

// getFrontendFS returns the embedded frontend filesystem
func getFrontendFS() (http.FileSystem, error) {
	if dir := utils.GetenvTrim("PULSE_FRONTEND_DIR"); dir != "" {
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

			serveIndexWithNonce(w, r, content)
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
					isImmutable := false

					if strings.HasSuffix(lookupPath, ".html") {
						// HTML files get nonce injection
						serveIndexWithNonce(w, r, content)
						return
					} else if strings.HasSuffix(lookupPath, ".css") {
						contentType = "text/css; charset=utf-8"
						// CSS files with hashes are immutable (e.g., index-abc123.css)
						isImmutable = strings.Contains(lookupPath, "-") && strings.Contains(lookupPath, ".css")
					} else if strings.HasSuffix(lookupPath, ".js") {
						contentType = "application/javascript; charset=utf-8"
						// JS files with hashes are immutable (e.g., index-BXHytNQV.js)
						isImmutable = strings.Contains(lookupPath, "-") && strings.Contains(lookupPath, ".js")
					} else if strings.HasSuffix(lookupPath, ".json") {
						contentType = "application/json"
					} else if strings.HasSuffix(lookupPath, ".svg") {
						contentType = "image/svg+xml"
					}

					w.Header().Set("Content-Type", contentType)

					// Hashed assets are immutable - cache aggressively
					// Non-hashed assets should not be cached
					if isImmutable {
						w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
					} else {
						w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
						w.Header().Set("Pragma", "no-cache")
						w.Header().Set("Expires", "0")
					}

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
					serveIndexWithNonce(w, r, content)
					return
				}
			}
		}

		// Not found
		http.NotFound(w, r)
	}
}
