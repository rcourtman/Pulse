package dockeragent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func TestLiveLogRequiresLocalPermissionAndExactContainer(t *testing.T) {
	id := strings.Repeat("a", 64)
	request := agentsdocker.LogTailRequest{ID: "34df03d1-9951-4418-a2e7-06750b2aecad", HostID: "runtime", ContainerID: id}
	replies := make(chan agentsdocker.LogTailResult, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var out agentsdocker.LogTailResult
		if err := json.NewDecoder(r.Body).Decode(&out); err != nil {
			t.Error(err)
		}
		replies <- out
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	exact := false
	reads := 0
	client := &fakeDockerClient{
		containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
			i := baseInspect()
			i.ID = strings.Repeat("b", 64)
			if exact {
				i.ID = id
			}
			return i, nil
		},
		containerLogTailFn: func(context.Context, string, int) (io.ReadCloser, error) {
			reads++
			return dockerLogFrames([2]string{"stdout", "started\n"}), nil
		},
	}
	a := &Agent{docker: client, httpClients: map[bool]*http.Client{false: server.Client()}}
	target := TargetConfig{URL: server.URL, Name: "primary", Authoritative: true}
	a.returnLogTail(context.Background(), target, request)
	if len(replies) != 0 || reads != 0 {
		t.Fatal("read without host opt-in")
	}
	a.cfg.AllowLiveLogs = true
	a.returnLogTail(context.Background(), target, request)
	if out := <-replies; !out.Unavailable || reads != 0 {
		t.Fatal("read a changed container")
	}
	exact = true
	a.returnLogTail(context.Background(), target, request)
	if out := <-replies; out.Unavailable || reads != 1 || len(out.Lines) != 1 || out.Lines[0] != "started" {
		t.Fatalf("unexpected tail: %+v", out)
	}
}
