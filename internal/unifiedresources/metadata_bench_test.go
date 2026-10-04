package unifiedresources

import "testing"

var benchmarkSensitivity ResourceSensitivity
var benchmarkScopes []string

// Component measurements of the per-read metadata path, not installed fleet
// CPU/write attribution or release qualification thresholds.
func BenchmarkRefreshPlatformScopes(b *testing.B) {
	resource := Resource{
		Type: ResourceTypeAppContainer, Sources: []DataSource{SourceDocker, SourceAgent},
		Agent: &AgentData{}, Docker: &DockerData{HostSourceID: "proxmox-lxc-docker:lab:node:100"},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RefreshPlatformScopes(&resource)
	}
	benchmarkScopes = resource.PlatformScopes
}

func BenchmarkClassifyResourceSensitivity(b *testing.B) {
	resource := Resource{Type: ResourceTypeVM, Tags: []string{"prod", "web", "database", "customer-data"}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkSensitivity = classifyResourceSensitivity(resource)
	}
}
