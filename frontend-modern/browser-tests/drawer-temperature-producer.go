// Read-only synthetic primary-CPU producer for the containing browser fixture.
// Run from the public module root. No monitor, database, API call or guest operation.
package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func main() {
	temperature := &models.Temperature{Available: true, HasCPU: true, CPUPackage: 55,
		Cores: []models.CoreTemp{{Core: 0, Temp: 50}}, HasGPU: true,
		GPU:  []models.GPUTemp{{Device: "fixture-gpu", Edge: 95}},
		NVMe: []models.NVMeTemp{{Device: "fixture-nvme", Temp: 91}}, LastUpdate: time.Now().UTC()}
	node := models.Node{ID: "fixture-pve1", Name: "pve1", Instance: "fixture", Type: "node", Status: "online", LastSeen: time.Now().UTC(), Temperature: temperature}
	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateFromSnapshot(models.StateSnapshot{Nodes: []models.Node{node}})
	resource, _, ok := adapter.GetByReference(node.ID)
	if !ok || resource.Temperature == nil || *resource.Temperature != 55 {
		panic("primary CPU projection did not preserve the supplied node")
	}
	proxmox, err := json.Marshal(resource.Proxmox)
	if err != nil {
		panic(err)
	}
	input := models.ResourceConvertInput{ID: resource.ID, Type: string(resource.Type), Name: resource.Name, Status: string(resource.Status), Temperature: resource.Temperature, Proxmox: proxmox, LastSeen: resource.LastSeen}
	frontend := models.ConvertResourceToFrontend(input)
	b, err := json.Marshal(frontend)
	if err != nil {
		panic(err)
	}
	fmt.Printf("DRAWER_TEMPERATURE_PRODUCER_JSON %s\n", b)
}
