package dockeragent

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/moby/moby/api/types/image"
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

	got, _, _, _ := agent.getImageRepoDigest(context.Background(), "image-id", "nginx:latest")
	if got != "" {
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

	got, _, _, _ := agent.getImageRepoDigest(context.Background(), "image-id", "nginx:latest")
	if got != "" {
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

	got, _, _, _ := agent.getImageRepoDigest(context.Background(), "image-id", "nginx:latest")
	if got != "sha256:abc" {
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

	got, _, _, _ := agent.getImageRepoDigest(context.Background(), "image-id", "custom:latest")
	if got != "sha256:first" {
		t.Fatalf("expected fallback digest, got %q", got)
	}
}

func TestAgent_getImageRepoDigests_MultipleDigestsForOneImage(t *testing.T) {
	agent := &Agent{
		docker: &fakeDockerClient{
			imageInspectWithRawFn: func(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
				return image.InspectResponse{
					RepoDigests: []string{
						"docker.io/library/postgres@sha256:44c4",
						"docker.io/library/postgres@sha256:cf78",
						"docker.io/other/postgres@sha256:beef",
					},
					Architecture: "amd64",
					Os:           "linux",
				}, nil, nil
			},
		},
		logger: zerolog.New(io.Discard),
	}

	digests, arch, os, variant := agent.getImageRepoDigests(context.Background(), "image-id", "postgres:16.15-alpine3.24")
	want := []string{"sha256:44c4", "sha256:cf78", "sha256:beef"}
	if len(digests) != len(want) {
		t.Fatalf("digests = %v, want %v", digests, want)
	}
	for i := range want {
		if digests[i] != want[i] {
			t.Fatalf("digests = %v, want %v", digests, want)
		}
	}
	if arch != "amd64" || os != "linux" || variant != "" {
		t.Fatalf("platform = %q/%q/%q, want amd64/linux/", arch, os, variant)
	}
	if got, _, _, _ := agent.getImageRepoDigest(context.Background(), "image-id", "postgres:16.15-alpine3.24"); got != "sha256:44c4" {
		t.Fatalf("primary digest = %q, want sha256:44c4", got)
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

	got, _, _, _ := agent.getImageRepoDigest(context.Background(), "image-id", "nginx:latest")
	if got != "" {
		t.Fatalf("expected empty digest for invalid repo digest, got %q", got)
	}
}
