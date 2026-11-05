package actionlifecycle

import (
	"context"
	"reflect"
	"testing"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type availabilityProjectionChecker struct {
	refused  map[string]string
	batches  int
	batchKey any
	sawBatch int
}

func (c *availabilityProjectionChecker) BeginAvailabilityBatch(ctx context.Context) context.Context {
	c.batches++
	return context.WithValue(ctx, &c.batchKey, true)
}

func (c *availabilityProjectionChecker) CheckActionAvailable(ctx context.Context, req unified.ActionRequest, _ unified.Resource) unified.ResourceActionReadiness {
	if ctx.Value(&c.batchKey) != nil {
		c.sawBatch++
	}
	if code, refused := c.refused[req.CapabilityName]; refused {
		return unified.ResourceActionReadiness{Name: req.CapabilityName, Available: false, ReasonCode: code}
	}
	return unified.ResourceActionReadiness{Name: req.CapabilityName, Available: true}
}

func projectionTestResource() unified.Resource {
	return unified.Resource{
		ID: "container-1",
		Capabilities: []unified.ResourceCapability{
			{Name: "start"}, {Name: "stop"}, {Name: "restart"},
		},
	}
}

func capabilityNames(resource unified.Resource) []string {
	names := make([]string, 0, len(resource.Capabilities))
	for _, capability := range resource.Capabilities {
		names = append(names, capability.Name)
	}
	return names
}

func TestProjectActionAvailabilityStripsWhatItRefuses(t *testing.T) {
	checker := &availabilityProjectionChecker{refused: map[string]string{"stop": "command_agent_disconnected"}}
	resources := []unified.Resource{projectionTestResource()}

	ProjectActionAvailability(context.Background(), checker, resources)

	if got, want := capabilityNames(resources[0]), []string{"start", "restart"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("capabilities = %v, want %v", got, want)
	}
	if len(resources[0].ActionReadiness) != 1 || resources[0].ActionReadiness[0].Name != "stop" {
		t.Fatalf("readiness = %#v, want the stop refusal", resources[0].ActionReadiness)
	}

	allRefused := &availabilityProjectionChecker{refused: map[string]string{"start": "x", "stop": "x", "restart": "x"}}
	resources = []unified.Resource{projectionTestResource()}
	ProjectActionAvailability(context.Background(), allRefused, resources)
	if resources[0].Capabilities != nil || len(resources[0].ActionReadiness) != 3 {
		t.Fatalf("all refused: capabilities = %#v, readiness = %#v", resources[0].Capabilities, resources[0].ActionReadiness)
	}
}

// A broadcast row keeps its capability set while its readiness moves: the
// operator policy form needs what a disconnected host offers, and a stable set
// keeps a refusal frame to one ref per row.
func TestAttachActionRefusalsKeepsCapabilitiesWhole(t *testing.T) {
	checker := &availabilityProjectionChecker{refused: map[string]string{"stop": "command_agent_disconnected", "restart": "command_agent_disconnected"}}
	shared := projectionTestResource()
	resources := []unified.Resource{shared, {ID: "container-2", Capabilities: []unified.ResourceCapability{{Name: "start"}}}, {ID: "no-capabilities"}}

	AttachActionRefusals(context.Background(), checker, resources)

	if got, want := capabilityNames(resources[0]), []string{"start", "stop", "restart"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("capabilities = %v, want %v", got, want)
	}
	if &resources[0].Capabilities[0] != &shared.Capabilities[0] {
		t.Fatal("capabilities were copied; a broadcast row must keep the shared slice")
	}
	if got := resources[0].ActionReadiness; len(got) != 2 || got[0].Name != "stop" || got[1].Name != "restart" {
		t.Fatalf("readiness = %#v, want stop and restart refusals", got)
	}
	if resources[1].ActionReadiness != nil || resources[2].ActionReadiness != nil {
		t.Fatalf("rows with nothing refused gained readiness: %#v, %#v", resources[1].ActionReadiness, resources[2].ActionReadiness)
	}
}

func TestAvailabilityProjectionsRunAsOneBatch(t *testing.T) {
	for name, project := range map[string]func(context.Context, AvailabilityChecker, []unified.Resource){
		"rest":      ProjectActionAvailability,
		"broadcast": AttachActionRefusals,
	} {
		t.Run(name, func(t *testing.T) {
			checker := &availabilityProjectionChecker{}
			project(context.Background(), checker, []unified.Resource{projectionTestResource(), projectionTestResource()})
			if checker.batches != 1 {
				t.Fatalf("batches = %d, want 1 per pass", checker.batches)
			}
			if checker.sawBatch != 6 {
				t.Fatalf("checks inside the batch = %d, want all 6", checker.sawBatch)
			}
		})
	}
}

func TestAvailabilityProjectionsToleratesNoChecker(t *testing.T) {
	resources := []unified.Resource{projectionTestResource()}
	ProjectActionAvailability(context.Background(), nil, resources)
	AttachActionRefusals(context.Background(), nil, resources)
	if len(resources[0].Capabilities) != 3 || resources[0].ActionReadiness != nil {
		t.Fatalf("no checker changed the row: %#v", resources[0])
	}
}
