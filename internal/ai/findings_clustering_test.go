package ai

import (
	"strings"
	"testing"
)

func TestFindingsStore_GetFindingClusters(t *testing.T) {
	store := NewFindingsStore()

	// 1. Cluster A: Performance issues on res-1 (2 findings)
	store.Add(&Finding{
		ID:          "performance-finding-1",
		ResourceID:  "res-1",
		Category:    FindingCategoryPerformance,
		Title:       "High CPU",
		Description: "CPU dangerously high",
		Severity:    FindingSeverityWarning,
	})
	store.Add(&Finding{
		ID:          "performance-finding-2",
		ResourceID:  "res-1",
		Category:    FindingCategoryPerformance,
		Title:       "High Memory",
		Description: "Memory dangerously high",
		Severity:    FindingSeverityCritical,
	})
	// These share: Resource (+0.3), Category (+0.2), "high"/"dangerously" overlap (~0.2+) -> >0.7

	// 2. Cluster B: Security issue on res-2 (1 finding)
	store.Add(&Finding{
		ID:          "security-finding-1",
		ResourceID:  "res-2",
		Category:    FindingCategorySecurity,
		Title:       "Login failed",
		Description: "Auth error",
		Severity:    FindingSeverityInfo,
	})

	// 3. Independent issue on res-1 but different category (should not cluster with A if threshold high enough)
	store.Add(&Finding{
		ID:          "backup-finding-1",
		ResourceID:  "res-1",
		Category:    FindingCategoryBackup,
		Title:       "Backup failed",
		Description: "Snapshot error",
		Severity:    FindingSeverityWatch,
	})
	// Similarity with perf1: Resource (+0.3), Category mismatch (0), Keywords (0) -> 0.3.

	clusters := store.GetFindingClusters(0.6)

	if len(clusters) != 3 {
		t.Fatalf("Expected 3 clusters, got %d", len(clusters))
	}

	// Verify Cluster A (Perf on res-1)
	var perfCluster *FindingCluster
	for _, c := range clusters {
		if c.CommonResource == "res-1" && c.CommonCategory == FindingCategoryPerformance {
			perfCluster = c
			break
		}
	}

	if perfCluster == nil {
		t.Fatal("Performance cluster for res-1 not found")
	}
	if perfCluster.TotalCount != 2 {
		t.Errorf("Expected perf cluster to have 2 findings, got %d", perfCluster.TotalCount)
	}
	if perfCluster.HighestSeverity != FindingSeverityCritical {
		t.Errorf("Expected perf cluster severity to be Critical, got %v", perfCluster.HighestSeverity)
	}
	if !strings.Contains(perfCluster.Summary, "related performance findings") {
		t.Errorf("Unexpected summary: %s", perfCluster.Summary)
	}

	// Verify Cluster B (Security on res-2)
	var secCluster *FindingCluster
	for _, c := range clusters {
		if c.CommonResource == "res-2" {
			secCluster = c
			break
		}
	}
	if secCluster == nil {
		t.Fatal("Security cluster for res-2 not found")
	}
	if secCluster.TotalCount != 1 {
		t.Errorf("Expected sec cluster to have 1 finding, got %d", secCluster.TotalCount)
	}
	if secCluster.Summary != "Login failed" {
		t.Errorf("Single finding cluster should use title as summary, got: %s", secCluster.Summary)
	}

	// Verify Cluster C (Backup on res-1)
	var bacCluster *FindingCluster
	for _, c := range clusters {
		if c.CommonResource == "res-1" && c.CommonCategory == FindingCategoryBackup {
			bacCluster = c
			break
		}
	}
	if bacCluster == nil {
		t.Fatal("Backup cluster for res-1 not found")
	}
	if bacCluster.TotalCount != 1 {
		t.Errorf("Expected backup cluster to have 1 finding, got %d", bacCluster.TotalCount)
	}
}

func TestFindingsStore_GetFindingClusters_MixedResources(t *testing.T) {
	store := NewFindingsStore()

	// 1. "Disk Full" issue on multiple servers. IDs > 8 chars.
	store.Add(&Finding{
		ID:          "capacity-finding-1",
		ResourceID:  "res-1",
		Category:    FindingCategoryCapacity,
		Title:       "Disk Full",
		Description: "Root volume is 100% full",
		Severity:    FindingSeverityCritical,
	})
	store.Add(&Finding{
		ID:          "capacity-finding-2",
		ResourceID:  "res-2",
		Category:    FindingCategoryCapacity,
		Title:       "Disk Full",
		Description: "Root volume is 100% full",
		Severity:    FindingSeverityWarning,
	})
	// Similarity: Category (0.2), Title (0.2*1=0.2), Description (0.1*1=0.1) -> 0.5
	// Resource mismatch (0)
	// Total 0.5.

	// If we set threshold to 0.4, they should cluster.

	clusters := store.GetFindingClusters(0.4)

	if len(clusters) != 1 {
		t.Fatalf("Expected 1 cluster, got %d", len(clusters))
	}

	c := clusters[0]
	if c.TotalCount != 2 {
		t.Errorf("Expected 2 findings in cluster, got %d", c.TotalCount)
	}
	if c.CommonResource != "" {
		t.Errorf("Expected CommonResource to be empty (mixed resources), got %s", c.CommonResource)
	}
	if c.HighestSeverity != FindingSeverityCritical {
		t.Errorf("Expected cluster severity to pick highest (Critical), got %v", c.HighestSeverity)
	}
}
