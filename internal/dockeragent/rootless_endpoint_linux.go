//go:build linux

package dockeragent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var (
	rootlessRuntimeRoot = "/run/user"
	effectiveUID        = os.Geteuid
)

func collectorOwnsRootlessEndpoint(endpoint string) bool {
	const unixPrefix = "unix://"
	if !strings.HasPrefix(endpoint, unixPrefix) {
		return false
	}
	path := strings.TrimPrefix(endpoint, unixPrefix)
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	uid := effectiveUID()
	root := filepath.Join(rootlessRuntimeRoot, strconv.Itoa(uid))
	dockerPath := filepath.Join(root, "docker.sock")
	podmanPath := filepath.Join(root, "podman", "podman.sock")
	if path != dockerPath && path != podmanPath {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	if info.Mode().Perm()&0o600 != 0o600 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(uid)
}

// collectorRootlessRuntimeCandidates returns one exact, live runtime endpoint
// owned by the collector. A typed-helper collector must never probe a rootful,
// remote, cross-user, or ambiguous daemon endpoint. Explicit environment pins
// resolve automatic discovery, but conflicting live pins still fail closed.
func collectorRootlessRuntimeCandidates(preference RuntimeKind) ([]runtimeCandidate, error) {
	uidRoot := filepath.Join(rootlessRuntimeRoot, strconv.Itoa(effectiveUID()))
	dockerURI := "unix://" + filepath.Join(uidRoot, "docker.sock")
	podmanURI := "unix://" + filepath.Join(uidRoot, "podman", "podman.sock")
	configured := make([]runtimeCandidate, 0, 3)
	addConfigured := func(label, value, expected string) error {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil
		}
		if value != expected {
			return fmt.Errorf("%s must pin the exact collector-owned endpoint %q", label, expected)
		}
		configured = append(configured, runtimeCandidate{host: value, label: label})
		return nil
	}

	var configErr error
	switch preference {
	case RuntimeDocker:
		configErr = addConfigured("DOCKER_HOST", os.Getenv("DOCKER_HOST"), dockerURI)
	case RuntimePodman:
		configErr = errors.Join(
			addConfigured("PODMAN_HOST", os.Getenv("PODMAN_HOST"), podmanURI),
			addConfigured("CONTAINER_HOST", os.Getenv("CONTAINER_HOST"), podmanURI),
		)
	default:
		configErr = errors.Join(
			addConfigured("DOCKER_HOST", os.Getenv("DOCKER_HOST"), dockerURI),
			addConfigured("PODMAN_HOST", os.Getenv("PODMAN_HOST"), podmanURI),
			addConfigured("CONTAINER_HOST", os.Getenv("CONTAINER_HOST"), podmanURI),
		)
	}
	if configErr != nil {
		return nil, configErr
	}

	candidates := configured
	if len(candidates) == 0 {
		switch preference {
		case RuntimeDocker:
			candidates = append(candidates, runtimeCandidate{
				host:  "unix://" + filepath.Join(uidRoot, "docker.sock"),
				label: "collector rootless docker socket",
			})
		case RuntimePodman:
			candidates = append(candidates, runtimeCandidate{
				host:  "unix://" + filepath.Join(uidRoot, "podman", "podman.sock"),
				label: "collector rootless podman socket",
			})
		default:
			candidates = append(candidates,
				runtimeCandidate{
					host:  "unix://" + filepath.Join(uidRoot, "docker.sock"),
					label: "collector rootless docker socket",
				},
				runtimeCandidate{
					host:  "unix://" + filepath.Join(uidRoot, "podman", "podman.sock"),
					label: "collector rootless podman socket",
				},
			)
		}
	}
	configuredEndpoints := make(map[string]struct{}, len(configured))
	for _, candidate := range configured {
		configuredEndpoints[candidate.host] = struct{}{}
	}
	if len(configuredEndpoints) > 1 {
		return nil, fmt.Errorf("conflicting collector rootless runtime pins are configured")
	}

	live := make([]runtimeCandidate, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if _, exists := seen[candidate.host]; exists {
			continue
		}
		seen[candidate.host] = struct{}{}
		if collectorOwnsRootlessEndpoint(candidate.host) {
			live = append(live, candidate)
		}
	}

	switch len(live) {
	case 0:
		return nil, fmt.Errorf("no live collector-owned rootless %s endpoint is available", preference)
	case 1:
		return live, nil
	default:
		endpoints := make([]string, 0, len(live))
		for _, candidate := range live {
			endpoints = append(endpoints, candidate.host)
		}
		return nil, fmt.Errorf("ambiguous collector-owned rootless runtime endpoints: %s", strings.Join(endpoints, ", "))
	}
}
