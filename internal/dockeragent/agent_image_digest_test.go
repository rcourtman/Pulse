package dockeragent

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/rs/zerolog"
)

func TestAgent_getImageRepoDigest_Error(t *testing.T) {
	agent := &Agent{
		docker: &fakeDockerClient{
			imageInspectWithRawFn: func(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
				return image.InspectResponse{}, nil, errors.New("inspect failed")
			},
		},
		logger: zerolog.New(io.Discard),
	}

	if got := agent.getImageRepoDigest(context.Background(), "image-id", "nginx:latest"); got != "" {
		t.Fatalf("expected empty digest on error, got %q", got)
	}
}

func TestAgent_getImageRepoDigest_NoRepoDigests(t *testing.T) {
	agent := &Agent{
		docker: &fakeDockerClient{
			imageInspectWithRawFn: func(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
				return image.InspectResponse{RepoDigests: nil}, nil, nil
			},
		},
		logger: zerolog.New(io.Discard),
	}

	if got := agent.getImageRepoDigest(context.Background(), "image-id", "nginx:latest"); got != "" {
		t.Fatalf("expected empty digest for no RepoDigests, got %q", got)
	}
}

func TestAgent_getImageRepoDigest_Match(t *testing.T) {
	agent := &Agent{
		docker: &fakeDockerClient{
			imageInspectWithRawFn: func(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
				return image.InspectResponse{RepoDigests: []string{"docker.io/library/nginx@sha256:abc"}}, nil, nil
			},
		},
		logger: zerolog.New(io.Discard),
	}

	if got := agent.getImageRepoDigest(context.Background(), "image-id", "nginx:latest"); got != "sha256:abc" {
		t.Fatalf("expected matching digest, got %q", got)
	}
}

func TestAgent_getImageRepoDigest_FallbackToFirst(t *testing.T) {
	agent := &Agent{
		docker: &fakeDockerClient{
			imageInspectWithRawFn: func(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
				return image.InspectResponse{RepoDigests: []string{
					"docker.io/library/redis@sha256:first",
					"docker.io/library/nginx@sha256:second",
				}}, nil, nil
			},
		},
		logger: zerolog.New(io.Discard),
	}

	if got := agent.getImageRepoDigest(context.Background(), "image-id", "custom:latest"); got != "sha256:first" {
		t.Fatalf("expected fallback digest, got %q", got)
	}
}

func TestAgent_getImageRepoDigest_InvalidRepoDigest(t *testing.T) {
	agent := &Agent{
		docker: &fakeDockerClient{
			imageInspectWithRawFn: func(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
				return image.InspectResponse{RepoDigests: []string{"invalid-digest"}}, nil, nil
			},
		},
		logger: zerolog.New(io.Discard),
	}

	if got := agent.getImageRepoDigest(context.Background(), "image-id", "nginx:latest"); got != "" {
		t.Fatalf("expected empty digest for invalid repo digest, got %q", got)
	}
}
