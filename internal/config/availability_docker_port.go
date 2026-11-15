package config

import (
	"fmt"
	"strings"
)

// AvailabilityDockerPortSource records that a check was created from a
// container's published port. A recreated container gets a new Docker ID, and
// with it a new resource ID, so the check keeps the host and the container
// name, which Docker holds unique per host. Pulse uses it to re-attach the
// check to the namesake and to tell which published ports already have one.
type AvailabilityDockerPortSource struct {
	HostID        string `json:"hostId"`
	ContainerName string `json:"containerName"`
	PrivatePort   int    `json:"privatePort"`
}

// Key identifies one published port of one named container on one host.
func (s AvailabilityDockerPortSource) Key() string {
	return fmt.Sprintf("%s\x00%s\x00%d", strings.TrimSpace(s.HostID), NormalizeDockerContainerName(s.ContainerName), s.PrivatePort)
}

// NormalizeDockerContainerName drops the leading slash Docker's API puts on
// container names, so "/web" and "web" name the same container.
func NormalizeDockerContainerName(name string) string {
	return strings.TrimPrefix(strings.TrimSpace(name), "/")
}

func normalizeAvailabilityDockerPortSource(source *AvailabilityDockerPortSource) *AvailabilityDockerPortSource {
	if source == nil {
		return nil
	}
	normalized := AvailabilityDockerPortSource{
		HostID:        strings.TrimSpace(source.HostID),
		ContainerName: NormalizeDockerContainerName(source.ContainerName),
		PrivatePort:   source.PrivatePort,
	}
	return &normalized
}

func validateAvailabilityDockerPortSource(source *AvailabilityDockerPortSource) error {
	if source == nil {
		return nil
	}
	if strings.TrimSpace(source.HostID) == "" || NormalizeDockerContainerName(source.ContainerName) == "" {
		return fmt.Errorf("a check created from a container port needs its host and container name")
	}
	if source.PrivatePort <= 0 || source.PrivatePort > 65535 {
		return fmt.Errorf("a check created from a container port needs a container port between 1 and 65535")
	}
	return nil
}
