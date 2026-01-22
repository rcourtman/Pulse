package tools

import (
"context"
"encoding/json"
"net/http"
"net/http/httptest"
"testing"

"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
"github.com/rcourtman/pulse-go-rewrite/internal/models"
"github.com/stretchr/testify/mock"
)

func TestExecuteGetCapabilities(t *testing.T) {
stateProv := &mockStateProvider{}
agentSrv := &mockAgentServer{}
agentSrv.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{
ame: "host1", Version: "1.0", Platform: "linux"},
})
executor := NewPulseToolExecutor(ExecutorConfig{
tServer:   agentSrv,
&mockMetricsHistoryProvider{},
eProvider: &BaselineMCPAdapter{},
Provider:  &PatternMCPAdapter{},
&mockAlertProvider{},
dingsProvider: &mockFindingsProvider{},
trolLevel:     ControlLevelControlled,
g{"100"},
})

result, err := executor.executeGetCapabilities(context.Background())
if err != nil {
expected error: %v", err)
}

var response CapabilitiesResponse
if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
se: %v", err)
}
if response.ControlLevel != string(ControlLevelControlled) || response.ConnectedAgents != 1 {
expected response: %+v", response)
}
if !response.Features.Control || !response.Features.MetricsHistory {
expected features: %+v", response.Features)
}
}

func TestExecuteGetURLContent(t *testing.T) {
	t.Setenv("PULSE_AI_ALLOW_LOOPBACK", "true")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
:= NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}})

if result, _ := executor.executeGetURLContent(context.Background(), map[string]interface{}{}); !result.IsError {
 url missing")
}

result, err := executor.executeGetURLContent(context.Background(), map[string]interface{}{
nil {
expected error: %v", err)
}
var response URLFetchResponse
if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
se: %v", err)
}
if response.StatusCode != http.StatusOK || response.Headers["X-Test"] != "ok" {
expected response: %+v", response)
}
}

func TestExecuteListInfrastructureAndTopology(t *testing.T) {
state := models.StateSnapshot{
odes: []models.Node{{ID: "node1", Name: "node1", Status: "online"}},
ame: "vm1", VMID: 100, Status: "running", Node: "node1"},
tainers: []models.Container{
ame: "ct1", VMID: 200, Status: "stopped", Node: "node1"},
       "host1",
ame:    "h1",
ame: "Host 1",
tainers: []models.DockerContainer{
ame: "nginx", State: "running", Image: "nginx"},
("GetState").Return(state)
agentSrv := &mockAgentServer{}
agentSrv.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{{Hostname: "node1"}})

executor := NewPulseToolExecutor(ExecutorConfig{
tServer:   agentSrv,
trolLevel:  ControlLevelControlled,
})

result, err := executor.executeListInfrastructure(context.Background(), map[string]interface{}{
"vms",
ning",
})
if err != nil {
expected error: %v", err)
}
var infra InfrastructureResponse
if err := json.Unmarshal([]byte(result.Content[0].Text), &infra); err != nil {
fra: %v", err)
}
if len(infra.VMs) != 1 || infra.VMs[0].Name != "vm1" {
expected infra response: %+v", infra)
}

// Topology includes derived node for VM reference if missing
state.Nodes = nil
stateProv2 := &mockStateProvider{}
stateProv2.On("GetState").Return(state)
executor.stateProvider = stateProv2
topologyResult, err := executor.executeGetTopology(context.Background(), map[string]interface{}{})
if err != nil {
expected error: %v", err)
}
var topology TopologyResponse
if err := json.Unmarshal([]byte(topologyResult.Content[0].Text), &topology); err != nil {
err)
}
if topology.Summary.TotalVMs != 1 || len(topology.Proxmox.Nodes) == 0 {
expected topology: %+v", topology)
}
}

func TestExecuteGetTopologySummaryOnly(t *testing.T) {
state := models.StateSnapshot{
odes: []models.Node{{ID: "node1", Name: "node1", Status: "online"}},
ame: "vm1", VMID: 100, Status: "running", Node: "node1"},
tainers: []models.Container{
ame: "ct1", VMID: 200, Status: "stopped", Node: "node1"},
ame: "host1",
tainers: []models.DockerContainer{
ame: "nginx", State: "running", Image: "nginx"},
("GetState").Return(state)
executor := NewPulseToolExecutor(ExecutorConfig{
executor.executeGetTopology(context.Background(), map[string]interface{}{
ly": true,
})
if err != nil {
expected error: %v", err)
}
var topology TopologyResponse
if err := json.Unmarshal([]byte(result.Content[0].Text), &topology); err != nil {
err)
}
if len(topology.Proxmox.Nodes) != 0 {
o proxmox nodes, got: %+v", topology.Proxmox.Nodes)
}
if len(topology.Docker.Hosts) != 0 {
o docker hosts, got: %+v", topology.Docker.Hosts)
}
if topology.Summary.TotalVMs != 1 || topology.Summary.TotalDockerHosts != 1 || topology.Summary.TotalDockerContainers != 1 {
expected summary: %+v", topology.Summary)
}
if topology.Summary.RunningVMs != 1 || topology.Summary.RunningDocker != 1 {
expected running summary: %+v", topology.Summary)
}
}

func TestExecuteSearchResources(t *testing.T) {
state := models.StateSnapshot{
odes: []models.Node{{ID: "node1", Name: "node1", Status: "online"}},
100, Name: "web-vm", Status: "running", Node: "node1"},
tainers: []models.Container{
Name: "db-ct", Status: "stopped", Node: "node1"},
       "host1",
ame:    "dock1",
ame: "Dock 1",
  "online",
tainers: []models.DockerContainer{
ame: "nginx", State: "running", Image: "nginx:latest"},
("GetState").Return(state)
executor := NewPulseToolExecutor(ExecutorConfig{
executor.executeSearchResources(context.Background(), map[string]interface{}{
ginx",
})
if err != nil {
expected error: %v", err)
}
var response ResourceSearchResponse
if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
se: %v", err)
}
if len(response.Matches) != 1 || response.Matches[0].Type != "docker" || response.Matches[0].Name != "nginx" {
expected search response: %+v", response)
}

result, err = executor.executeSearchResources(context.Background(), map[string]interface{}{
 "vm",
})
if err != nil {
expected error: %v", err)
}
response = ResourceSearchResponse{}
if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
se: %v", err)
}
if len(response.Matches) != 1 || response.Matches[0].Type != "vm" || response.Matches[0].Name != "web-vm" {
expected search response: %+v", response)
}
}

func TestExecuteSetResourceURLAndGetResource(t *testing.T) {
executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}})

if result, _ := executor.executeSetResourceURL(context.Background(), map[string]interface{}{}); !result.IsError {
 resource_type missing")
}

updater := &fakeMetadataUpdater{}
executor.metadataUpdater = updater
result, err := executor.executeSetResourceURL(context.Background(), map[string]interface{}{
 "100",
       "http://example",
})
if err != nil {
expected error: %v", err)
}
if len(updater.resourceArgs) != 3 || updater.resourceArgs[2] != "http://example" {
expected updater args: %+v", updater.resourceArgs)
}
var setResp map[string]interface{}
if err := json.Unmarshal([]byte(result.Content[0].Text), &setResp); err != nil {
se: %v", err)
}
if setResp["action"] != "set" {
expected set response: %+v", setResp)
}

state := models.StateSnapshot{
    []models.VM{{ID: "vm1", VMID: 100, Name: "vm1", Status: "running", Node: "node1"}},
tainers: []models.Container{{ID: "ct1", VMID: 200, Name: "ct1", Status: "running", Node: "node1"}},
ame: "host",
tainers: []models.DockerContainer{{
"abc123",
ame:  "nginx",
ning",
ginx",
("GetState").Return(state)
executor.stateProvider = stateProv

resource, _ := executor.executeGetResource(context.Background(), map[string]interface{}{
 "100",
})
var res ResourceResponse
if err := json.Unmarshal([]byte(resource.Content[0].Text), &res); err != nil {
res.Type != "vm" || res.Name != "vm1" {
expected resource: %+v", res)
}

resource, _ = executor.executeGetResource(context.Background(), map[string]interface{}{
 "abc",
})
if err := json.Unmarshal([]byte(resource.Content[0].Text), &res); err != nil {
err)
}
if res.Type != "docker" || res.Name != "nginx" {
expected docker resource: %+v", res)
}
}

func TestIntArg(t *testing.T) {
if got := intArg(map[string]interface{}{}, "limit", 10); got != 10 {
expected default: %d", got)
}
if got := intArg(map[string]interface{}{"limit": float64(5)}, "limit", 10); got != 5 {
expected value: %d", got)
}
}
