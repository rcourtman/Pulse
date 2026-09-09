package dockeragent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// Exercise the real transport, not dockerClient mocks: dependency upgrades must
// negotiate older daemons and preserve recreate payloads at the new API ceiling.
func TestMobyNegotiatedContainerContracts(t *testing.T) {
	for _, version := range []string{"1.44", "1.51", "1.56"} {
		t.Run(version, func(t *testing.T) {
			var created container.CreateRequest
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/_ping":
					w.Header().Set("API-Version", version)
				case "/v" + version + "/containers/json":
					if r.Method != http.MethodGet || r.URL.Query().Get("all") != "1" {
						t.Errorf("list request: %s %s", r.Method, r.URL)
					}
					_, _ = w.Write([]byte(`[{"Id":"fixture","Names":["/workload"],"State":"running","Image":"example:test"}]`))
				case "/v" + version + "/containers/create":
					if r.Method != http.MethodPost || r.URL.Query().Get("name") != "workload" {
						t.Errorf("create request: %s %s", r.Method, r.URL)
					}
					if err := json.NewDecoder(r.Body).Decode(&created); err != nil {
						t.Error(err)
					}
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"Id":"replacement","Warnings":[]}`))
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					http.Error(w, "unexpected", http.StatusNotFound)
				}
			}))
			defer server.Close()
			cli, err := client.New(client.WithHost(server.URL), client.WithAPIVersionNegotiation())
			if err != nil {
				t.Fatal(err)
			}
			defer cli.Close()
			listed, err := cli.ContainerList(context.Background(), client.ContainerListOptions{All: true})
			if err != nil {
				t.Fatal(err)
			}
			if len(listed.Items) != 1 || listed.Items[0].ID != "fixture" {
				t.Fatalf("inventory: %+v", listed)
			}
			host := &container.HostConfig{Binds: []string{"persistent:/data"}, CapAdd: []string{"net_admin", "CAP_NET_ADMIN"}, CapDrop: []string{"sys_admin"}, RestartPolicy: container.RestartPolicy{Name: "unless-stopped"}}
			result, err := cli.ContainerCreate(context.Background(), client.ContainerCreateOptions{Name: "workload", Config: &container.Config{Image: "example:test", Env: []string{"MODE=test"}}, HostConfig: host})
			if err != nil {
				t.Fatal(err)
			}
			if result.ID != "replacement" || created.Image != "example:test" || !reflect.DeepEqual(created.Env, []string{"MODE=test"}) || created.HostConfig == nil {
				t.Fatalf("create result or config lost: %+v %+v", result, created)
			}
			if !reflect.DeepEqual(created.HostConfig.Binds, host.Binds) || created.HostConfig.RestartPolicy != host.RestartPolicy || !reflect.DeepEqual(created.HostConfig.CapAdd, []string{"CAP_NET_ADMIN"}) || !reflect.DeepEqual(created.HostConfig.CapDrop, []string{"CAP_SYS_ADMIN"}) {
				t.Fatalf("host configuration lost: %+v", created.HostConfig)
			}
			if !reflect.DeepEqual(host.CapAdd, []string{"net_admin", "CAP_NET_ADMIN"}) || !reflect.DeepEqual(host.CapDrop, []string{"sys_admin"}) {
				t.Fatalf("client mutated caller-owned recreation config: %+v", host)
			}
		})
	}
}
