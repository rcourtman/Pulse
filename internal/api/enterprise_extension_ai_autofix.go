package api

import (
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

var (
	aiAutoFixBindMu          sync.RWMutex
	aiAutoFixEndpointsBinder extensions.BindAIAutoFixEndpointsFunc
)

// SetAIAutoFixEndpointsBinder registers a binder that can replace or decorate
// default AI auto-fix endpoint handlers.
func SetAIAutoFixEndpointsBinder(binder extensions.BindAIAutoFixEndpointsFunc) {
	aiAutoFixBindMu.Lock()
	defer aiAutoFixBindMu.Unlock()
	aiAutoFixEndpointsBinder = binder
}

func resolveAIAutoFixEndpoints(defaults extensions.AIAutoFixEndpoints, runtime extensions.AIAutoFixRuntime) extensions.AIAutoFixEndpoints {
	aiAutoFixBindMu.RLock()
	binder := aiAutoFixEndpointsBinder
	aiAutoFixBindMu.RUnlock()

	if binder == nil || defaults == nil {
		return defaults
	}

	resolved := binder(defaults, runtime)
	if resolved == nil {
		return defaults
	}

	return resolved
}
