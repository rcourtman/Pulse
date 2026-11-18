package api

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"strings"
	"testing"
)

func TestDockerLiveLogOnlyReturnsToWaitingRuntimeReader(t *testing.T) {
	id := strings.Repeat("a", 64)
	host := models.DockerHost{ID: "runtime", TokenID: "reporter", LiveLogsEnabled: true, Containers: []models.DockerContainer{{ID: id}}}
	var broker dockerLiveLogRequests
	slot := broker.start("org", host, id)
	if slot == nil || broker.start("org", host, id) != nil {
		t.Fatal("expected exactly one waiting read")
	}
	for _, identity := range [][2]string{{"other", "reporter"}, {"org", "other"}} {
		if broker.offer(identity[0], host, identity[1]) != nil {
			t.Fatal("request crossed reporting identity")
		}
	}
	req := broker.offer("org", host, "reporter")
	if req == nil || req.ContainerID != id {
		t.Fatal("missing exact read")
	}
	reply := agentsdocker.LogTailResult{LogTailRequest: *req, Lines: []string{strings.Repeat("x", 5000)}}
	if broker.finish("other", "reporter", reply) || broker.finish("org", "other", reply) {
		t.Fatal("result crossed identity")
	}
	if !broker.finish("org", "reporter", reply) || broker.finish("org", "reporter", reply) {
		t.Fatal("expected one result")
	}
	received := <-slot.result
	if !received.Truncated || len(received.Lines[0]) > agentsdocker.FailureLogMaxLineBytes {
		t.Fatal("unbounded result")
	}
	next := broker.start("org", host, id)
	if next == nil {
		t.Fatal("expected reader to be released")
	}
	broker.remove(next.request.ID)
	if broker.offer("org", host, "reporter") != nil {
		t.Fatal("disconnected read was delivered")
	}
}
