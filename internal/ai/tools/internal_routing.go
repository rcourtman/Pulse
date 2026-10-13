package tools

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func (e *PulseToolExecutor) resolveResourceLocation(reference string) models.ResourceLocation {
	rs, err := e.readStateForControl()
	if err != nil {
		return models.ResourceLocation{Name: reference}
	}
	if selected := unifiedresources.ResolveResourceReferenceContext(rs, reference); selected.Resource != nil {
		return selected.Location
	}
	return unifiedresources.ResolveResourceContext(rs, reference).Location
}
