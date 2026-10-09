package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type identityGuestConfigProvider struct {
	mockGuestConfigProvider
	calls int
}

func (p *identityGuestConfigProvider) GetGuestConfig(kind, instance, node string, vmid int) (map[string]interface{}, error) {
	p.calls++
	return p.mockGuestConfigProvider.GetGuestConfig(kind, instance, node, vmid)
}

func TestGuestConfigToolIdentity(t *testing.T) {
	for _, kind := range []string{"vm", "system-container"} {
		for _, reverse := range []bool{false, true} {
			for _, mode := range []string{"VMID collision", "name collision", "canonical ID", "ID shadows name", "unique VMID", "unique name", "missing", "missing node", "missing instance", "invalid VMID"} {
				t.Run(kind+"/"+mode+"/reverse="+map[bool]string{true: "yes", false: "no"}[reverse], func(t *testing.T) {
					vms := []models.VM{{ID: "first-id", VMID: 105, Name: "duplicate", Instance: "first", Node: "a"}, {ID: "second-id", VMID: 105, Name: "duplicate", Instance: "second", Node: "b", Tags: []string{"customer-data"}}}
					ref, failure := "second-id", ""
					switch mode {
					case "VMID collision":
						ref, failure = "105", "ambiguous"
					case "name collision":
						ref, failure = "duplicate", "ambiguous"
					case "ID shadows name":
						// Set after canonicalisation below, not to a source-only ID.
					case "unique VMID":
						vms[1].VMID, ref = 106, "106"
					case "unique name":
						vms[1].Name, ref = "selected", "selected"
					case "missing":
						ref, failure = "absent", "not found"
					case "missing node":
						vms[1].Node, failure = "", "placement is unavailable"
					case "missing instance":
						vms[1].Instance, failure = "", "placement is unavailable"
					case "invalid VMID":
						vms[1].VMID, failure = 0, "placement is unavailable"
					}
					wantVMID, wantName := vms[1].VMID, vms[1].Name
					if reverse {
						vms[0], vms[1] = vms[1], vms[0]
					}
					state := models.StateSnapshot{}
					if kind == "vm" {
						state.VMs = vms
					} else {
						for _, vm := range vms {
							state.Containers = append(state.Containers, models.Container{ID: vm.ID, VMID: vm.VMID, Name: vm.Name, Instance: vm.Instance, Node: vm.Node, Tags: vm.Tags})
						}
					}
					registry := unifiedresources.NewRegistry(nil)
					registry.IngestSnapshot(state)
					selectedID, selectedSummary := "", ""
					var selectedPolicy *unifiedresources.ResourcePolicy
					if kind == "vm" {
						for _, vm := range registry.VMs() {
							if vm.SourceID() == "second-id" {
								selectedID = vm.ID()
								selectedPolicy, selectedSummary = vm.GovernanceMetadata()
							}
						}
					} else {
						for _, ct := range registry.Containers() {
							if ct.SourceID() == "second-id" {
								selectedID = ct.ID()
								selectedPolicy, selectedSummary = ct.GovernanceMetadata()
							}
						}
					}
					if selectedID == "" {
						t.Fatal("selected canonical fixture is absent")
					}
					if ref == "second-id" {
						ref = selectedID
					}
					if mode == "ID shadows name" {
						for i := range state.VMs {
							if state.VMs[i].ID == "first-id" {
								state.VMs[i].Name = selectedID
							}
						}
						for i := range state.Containers {
							if state.Containers[i].ID == "first-id" {
								state.Containers[i].Name = selectedID
							}
						}
					}
					provider := &identityGuestConfigProvider{mockGuestConfigProvider: mockGuestConfigProvider{config: map[string]interface{}{"ostype": "fixture"}}}
					executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}, GuestConfigProvider: provider})
					result, err := executor.executeGetResourceConfig(context.Background(), map[string]interface{}{"resource_type": kind, "resource_id": ref})
					if err != nil {
						t.Fatal(err)
					}
					if failure != "" {
						if !result.IsError || !strings.Contains(result.Content[0].Text, failure) || provider.calls != 0 {
							t.Fatalf("ambiguous/incomplete target reached provider: calls=%d result=%+v", provider.calls, result)
						}
						return
					}
					if result.IsError || provider.calls != 1 || provider.lastInstance != "second" || provider.lastNode != "b" || provider.lastVMID != wantVMID {
						t.Fatalf("wrong provider target: %+v result=%+v", provider, result)
					}
					var response GuestConfigResponse
					if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
						t.Fatal(err)
					}
					if response.Instance != "second" || response.Node != "b" || response.VMID != wantVMID || response.Name != wantName || response.OSType != "fixture" || response.Policy == nil || response.Policy.Sensitivity != selectedPolicy.Sensitivity || response.AISafeSummary != selectedSummary {
						t.Fatalf("wrong config attribution: %+v", response)
					}
				})
			}
		}
	}
}
