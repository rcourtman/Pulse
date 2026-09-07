//go:build !linux

package filesystemprobe

import (
	"context"
	"errors"

	"github.com/rcourtman/pulse-go-rewrite/pkg/agents/filesystem"
)

func observeContainer(context.Context, ContainerRequest) ([]filesystem.Observation, error) {
	return nil, errors.New("container filesystem observations require access to the Linux process namespace")
}
