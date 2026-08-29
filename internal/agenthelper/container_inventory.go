package agenthelper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
)

const (
	maxContainerDaemonBytes = 8 * 1024 * 1024
	maxContainerCount       = 2048
	maxContainerString      = 1024
)

type ContainerEndpoint struct {
	Runtime    string
	SocketPath string
	APIPath    string
}

type LocalContainerProvider struct {
	endpoints []ContainerEndpoint
	dial      func(context.Context, string) (net.Conn, error)
}

type ContainerInventoryResult struct {
	Runtimes []ContainerRuntimeSnapshot `json:"runtimes"`
}

type ContainerRuntimeSnapshot struct {
	Runtime    string             `json:"runtime"`
	Available  bool               `json:"available"`
	Containers []ContainerSummary `json:"containers"`
	ErrorCode  string             `json:"errorCode,omitempty"`
}

type ContainerSummary struct {
	ID      string   `json:"id"`
	Names   []string `json:"names,omitempty"`
	Image   string   `json:"image,omitempty"`
	State   string   `json:"state,omitempty"`
	Status  string   `json:"status,omitempty"`
	Created int64    `json:"created,omitempty"`
}

type daemonContainer struct {
	ID      string   `json:"Id"`
	Names   []string `json:"Names"`
	Image   string   `json:"Image"`
	State   string   `json:"State"`
	Status  string   `json:"Status"`
	Created int64    `json:"Created"`
}

func NewLocalContainerProvider(endpoints []ContainerEndpoint) (*LocalContainerProvider, error) {
	if len(endpoints) == 0 || len(endpoints) > 2 {
		return nil, errors.New("one or two fixed container endpoints are required")
	}
	seen := make(map[string]struct{}, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint.Runtime != "docker" && endpoint.Runtime != "podman" {
			return nil, fmt.Errorf("unsupported container runtime %q", endpoint.Runtime)
		}
		if _, ok := seen[endpoint.Runtime]; ok {
			return nil, fmt.Errorf("duplicate container runtime %q", endpoint.Runtime)
		}
		seen[endpoint.Runtime] = struct{}{}
		if endpoint.SocketPath == "" || endpoint.SocketPath[0] != '/' || endpoint.APIPath == "" || endpoint.APIPath[0] != '/' {
			return nil, errors.New("container endpoints must use fixed absolute socket and API paths")
		}
		if strings.Contains(endpoint.SocketPath, "..") || strings.Contains(endpoint.APIPath, "..") || strings.ContainsAny(endpoint.APIPath, "\r\n") {
			return nil, errors.New("container endpoint contains an unsafe path")
		}
	}
	return &LocalContainerProvider{endpoints: append([]ContainerEndpoint(nil), endpoints...), dial: dialFixedUnixSocket}, nil
}

func dialFixedUnixSocket(ctx context.Context, socketPath string) (net.Conn, error) {
	info, err := os.Lstat(socketPath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSocket == 0 || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("container endpoint is not a Unix socket")
	}
	dialer := net.Dialer{}
	return dialer.DialContext(ctx, "unix", socketPath)
}

func (p *LocalContainerProvider) Inventory(ctx context.Context) (json.RawMessage, error) {
	result := ContainerInventoryResult{Runtimes: make([]ContainerRuntimeSnapshot, 0, len(p.endpoints))}
	for _, endpoint := range p.endpoints {
		snapshot := ContainerRuntimeSnapshot{Runtime: endpoint.Runtime, Containers: []ContainerSummary{}}
		containers, err := p.inventoryEndpoint(ctx, endpoint)
		if err != nil {
			if ctx.Err() != nil {
				return nil, &ProviderError{Code: ErrorDeadlineExceeded, Message: "container inventory deadline exceeded", Retryable: true}
			}
			snapshot.ErrorCode = ErrorProviderUnavailable
		} else {
			snapshot.Available = true
			snapshot.Containers = containers
		}
		result.Runtimes = append(result.Runtimes, snapshot)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func (p *LocalContainerProvider) inventoryEndpoint(ctx context.Context, endpoint ContainerEndpoint) ([]ContainerSummary, error) {
	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(dialCtx context.Context, _, _ string) (net.Conn, error) {
			return p.dial(dialCtx, endpoint.SocketPath)
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://local-helper"+endpoint.APIPath, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("container runtime returned HTTP %d", response.StatusCode)
	}
	limited := io.LimitReader(response.Body, maxContainerDaemonBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(data) > maxContainerDaemonBytes {
		return nil, errors.New("container runtime response exceeds limit")
	}
	var raw []daemonContainer
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode container inventory: %w", err)
	}
	if len(raw) > maxContainerCount {
		return nil, errors.New("container runtime count exceeds limit")
	}
	containers := make([]ContainerSummary, 0, len(raw))
	for _, item := range raw {
		if !boundedContainerText(item.ID) || !boundedContainerText(item.Image) || !boundedContainerText(item.State) || !boundedContainerText(item.Status) {
			return nil, errors.New("container runtime field exceeds limit")
		}
		if len(item.Names) > 32 {
			return nil, errors.New("container runtime names exceed limit")
		}
		for _, name := range item.Names {
			if !boundedContainerText(name) {
				return nil, errors.New("container runtime name exceeds limit")
			}
		}
		containers = append(containers, ContainerSummary{
			ID: item.ID, Names: append([]string(nil), item.Names...), Image: item.Image,
			State: item.State, Status: item.Status, Created: item.Created,
		})
	}
	return containers, nil
}

func boundedContainerText(value string) bool {
	return len(value) <= maxContainerString && !strings.ContainsAny(value, "\x00\r\n")
}
