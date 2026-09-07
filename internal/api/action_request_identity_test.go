package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestActionPlanRequestConflictIsStableConflict(t *testing.T) {
	rec := httptest.NewRecorder()
	writeActionPlanError(rec, &actionlifecycle.PersistError{Op: "action request identity", Err: unified.ErrActionIdentityConflict})
	var body agentcapabilities.ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusConflict || body.Error != agentcapabilities.AgentErrCodeActionRequestConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
