package agenthelper

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestContainerInventoryUsesOnlyFixedBoundedGET(t *testing.T) {
	provider, err := NewLocalContainerProvider([]ContainerEndpoint{{
		Runtime: "docker", SocketPath: "/fixed/docker.sock", APIPath: "/v1.41/containers/json?all=1",
	}})
	if err != nil {
		t.Fatal(err)
	}
	provider.dial = func(_ context.Context, socket string) (net.Conn, error) {
		if socket != "/fixed/docker.sock" {
			t.Fatalf("dialed socket %q", socket)
		}
		server, client := net.Pipe()
		go func() {
			defer server.Close()
			request, readErr := http.ReadRequest(bufio.NewReader(server))
			if readErr != nil {
				return
			}
			if request.Method != http.MethodGet || request.URL.RequestURI() != "/v1.41/containers/json?all=1" {
				t.Errorf("daemon request = %s %s", request.Method, request.URL.RequestURI())
			}
			_, _ = fmt.Fprint(server, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nConnection: close\r\n\r\n"+
				`[{"Id":"abc","Names":["/pulse"],"Image":"pulse:1","State":"running","Status":"Up","Created":1,"Labels":{"ignored":"closed-output"}}]`)
		}()
		return client, nil
	}
	result, err := provider.Inventory(t.Context())
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if strings.Contains(string(result), "ignored") || !strings.Contains(string(result), `"id":"abc"`) {
		t.Fatalf("inventory did not project the closed schema: %s", result)
	}
}

func TestContainerInventoryBoundsDaemonOutput(t *testing.T) {
	const podmanDockerCompatPath = "/v1.40/containers/json?all=1"
	provider, err := NewLocalContainerProvider([]ContainerEndpoint{{Runtime: "podman", SocketPath: "/fixed/podman.sock", APIPath: podmanDockerCompatPath}})
	if err != nil {
		t.Fatal(err)
	}
	provider.dial = func(context.Context, string) (net.Conn, error) {
		server, client := net.Pipe()
		go func() {
			defer server.Close()
			request, readErr := http.ReadRequest(bufio.NewReader(server))
			if readErr != nil {
				return
			}
			if request.Method != http.MethodGet || request.URL.RequestURI() != podmanDockerCompatPath {
				t.Errorf("Podman compatibility request = %s %s", request.Method, request.URL.RequestURI())
			}
			_, _ = fmt.Fprint(server, "HTTP/1.1 200 OK\r\nConnection: close\r\n\r\n[\""+strings.Repeat("x", maxContainerDaemonBytes)+"\"]")
		}()
		return client, nil
	}
	result, err := provider.Inventory(t.Context())
	if err != nil {
		t.Fatalf("bounded endpoint failure should be a closed unavailable snapshot: %v", err)
	}
	if !strings.Contains(string(result), `"available":false`) || !strings.Contains(string(result), ErrorProviderUnavailable) {
		t.Fatalf("oversized output was not bounded: %s", result)
	}
}

func TestContainerInventoryHonorsDeadline(t *testing.T) {
	provider, err := NewLocalContainerProvider([]ContainerEndpoint{{Runtime: "docker", SocketPath: "/fixed/docker.sock", APIPath: "/containers/json"}})
	if err != nil {
		t.Fatal(err)
	}
	provider.dial = func(ctx context.Context, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err = provider.Inventory(ctx)
	var typed *ProviderError
	if !errors.As(err, &typed) || typed.Code != ErrorDeadlineExceeded {
		t.Fatalf("deadline error = %T %v", err, err)
	}
}

func TestContainerEndpointRejectsCallerSelectedProxyShapes(t *testing.T) {
	for _, endpoint := range []ContainerEndpoint{
		{Runtime: "containerd", SocketPath: "/run/x.sock", APIPath: "/containers"},
		{Runtime: "docker", SocketPath: "relative.sock", APIPath: "/containers"},
		{Runtime: "docker", SocketPath: "/run/../tmp/x.sock", APIPath: "/containers"},
		{Runtime: "docker", SocketPath: "/run/x.sock", APIPath: "http://attacker/"},
	} {
		if _, err := NewLocalContainerProvider([]ContainerEndpoint{endpoint}); err == nil {
			t.Fatalf("unsafe endpoint accepted: %#v", endpoint)
		}
	}
}
