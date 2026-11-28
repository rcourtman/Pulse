package dockeragent

import (
	"context"
	"io"
	"strings"
	"testing"
)

type pullStreamFakeDockerClient struct {
	*fakeDockerClient
	stream string
}

func (m *pullStreamFakeDockerClient) ImagePull(ctx context.Context, refStr string, options dockerImagePullOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(m.stream)), nil
}

func TestPullUpdateImageEmitsLayerMilestonesAndStatus(t *testing.T) {
	stream := strings.Join([]string{
		`{"status":"Pulling from qmcgaw/gluetun","id":"latest"}`,
		`{"status":"Pulling fs layer","id":"aaa111"}`,
		`{"status":"Pulling fs layer","id":"bbb222"}`,
		`{"status":"Downloading","id":"aaa111"}`,
		`{"status":"Download complete","id":"aaa111"}`,
		`{"status":"Pull complete","id":"aaa111"}`,
		`{"status":"Already exists","id":"bbb222"}`,
		`{"status":"Digest: sha256:1234567890abcdef"}`,
		`{"status":"Status: Downloaded newer image for qmcgaw/gluetun:latest"}`,
	}, "\n") + "\n"

	agent := &Agent{
		docker: &pullStreamFakeDockerClient{fakeDockerClient: &fakeDockerClient{}, stream: stream},
	}

	var steps []string
	err := agent.pullUpdateImageWithProgress(context.Background(), "qmcgaw/gluetun:latest", func(step string) {
		steps = append(steps, step)
	})
	if err != nil {
		t.Fatalf("pullUpdateImage returned error: %v", err)
	}

	joined := strings.Join(steps, " | ")
	for _, want := range []string{
		"Pulled 2/2 image layers",
		"Digest: sha256:1234567890abcdef",
		"Status: Downloaded newer image for qmcgaw/gluetun:latest",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in progress steps, got %v", want, steps)
		}
	}
}
