package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestMergeLinkedHostZFSDatasetsEnrichesAndSynthesizesPools(t *testing.T) {
	pools := map[string]*models.ZFSPool{
		"rpool": {Name: "rpool", State: "ONLINE"},
	}
	host := &models.Host{ZFSPools: []models.HostZFSPool{
		{
			Name: "rpool",
			Datasets: []models.ZFSDataset{
				{Name: "rpool/data", Type: "filesystem", UsedBytes: 100},
			},
		},
		{
			Name: "tank",
			Datasets: []models.ZFSDataset{
				{Name: "tank/vm", Type: "volume", UsedBytes: 200},
			},
		},
	}}

	mergeLinkedHostZFSDatasets(pools, host)

	if len(pools["rpool"].Datasets) != 1 || pools["rpool"].State != "ONLINE" {
		t.Fatalf("existing pool not enriched safely: %#v", pools["rpool"])
	}
	if pools["tank"] == nil || pools["tank"].State != "UNKNOWN" ||
		len(pools["tank"].Datasets) != 1 {
		t.Fatalf("agent-only pool not synthesized: %#v", pools["tank"])
	}
}
