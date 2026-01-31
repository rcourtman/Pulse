package chat

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKnowledgeAccumulator_Basic(t *testing.T) {
	ka := NewKnowledgeAccumulator()
	assert.Equal(t, 0, ka.Len())
	assert.Equal(t, "", ka.Render())

	ka.AddFact(FactCategoryResource, "lxc:delly:106:status", "running, Postfix")
	assert.Equal(t, 1, ka.Len())

	rendered := ka.Render()
	assert.Contains(t, rendered, "Known Facts")
	assert.Contains(t, rendered, "Resources:")
	assert.Contains(t, rendered, "[lxc:delly:106:status] running, Postfix")
}

func TestKnowledgeAccumulator_Upsert(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	ka.AddFact(FactCategoryResource, "lxc:delly:106:status", "running, CPU=5%")
	assert.Equal(t, 1, ka.Len())

	// Upsert with same key should update value, not add a new entry
	ka.AddFact(FactCategoryResource, "lxc:delly:106:status", "stopped")
	assert.Equal(t, 1, ka.Len())

	rendered := ka.Render()
	assert.Contains(t, rendered, "stopped")
	assert.NotContains(t, rendered, "CPU=5%")
}

func TestKnowledgeAccumulator_EmptyKeyOrValue(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	ka.AddFact(FactCategoryResource, "", "some value")
	assert.Equal(t, 0, ka.Len())

	ka.AddFact(FactCategoryResource, "some-key", "")
	assert.Equal(t, 0, ka.Len())
}

func TestKnowledgeAccumulator_ValueTruncation(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	longValue := strings.Repeat("x", 300)
	ka.AddFact(FactCategoryExec, "exec:host:cmd", longValue)

	fact := ka.facts["exec:host:cmd"]
	require.NotNil(t, fact)
	assert.Equal(t, maxValueLen, len(fact.Value))
}

func TestKnowledgeAccumulator_MaxEntries(t *testing.T) {
	ka := NewKnowledgeAccumulator()
	ka.maxEntries = 5
	ka.maxChars = 100000 // High limit so entries are the constraint

	// Insert 5 facts at turn 0
	ka.SetTurn(0)
	for i := 0; i < 5; i++ {
		ka.AddFact(FactCategoryResource, keyForIndex(i), valForIndex(i))
	}
	assert.Equal(t, 5, ka.Len())

	// Move to turn 2 so turn 0 facts are evictable
	ka.SetTurn(2)
	ka.AddFact(FactCategoryResource, "new-key", "new-value")

	// Should have evicted one old fact to make room
	assert.Equal(t, 5, ka.Len())
	_, hasNew := ka.facts["new-key"]
	assert.True(t, hasNew, "new fact should be present")
}

func TestKnowledgeAccumulator_MaxChars(t *testing.T) {
	ka := NewKnowledgeAccumulator()
	ka.maxEntries = 1000
	ka.maxChars = 50 // Very low char budget

	ka.SetTurn(0)
	ka.AddFact(FactCategoryResource, "k1", strings.Repeat("a", 20))
	ka.AddFact(FactCategoryResource, "k2", strings.Repeat("b", 20))

	// Move forward so old facts can be evicted
	ka.SetTurn(2)
	ka.AddFact(FactCategoryResource, "k3", strings.Repeat("c", 20))

	// Should have evicted oldest to stay within budget
	assert.LessOrEqual(t, ka.TotalChars(), 50+maxValueLen, "should stay near char budget")
}

func TestKnowledgeAccumulator_SoftPinCurrentTurn(t *testing.T) {
	ka := NewKnowledgeAccumulator()
	ka.maxEntries = 2
	ka.maxChars = 100000

	// All facts in current turn should be soft-pinned
	ka.SetTurn(5)
	ka.AddFact(FactCategoryResource, "k1", "val1")
	ka.AddFact(FactCategoryResource, "k2", "val2")
	ka.AddFact(FactCategoryResource, "k3", "val3")

	// All 3 should survive even though maxEntries=2 because they're all from current turn
	assert.Equal(t, 3, ka.Len())
}

func TestKnowledgeAccumulator_CategoryGrouping(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	ka.AddFact(FactCategoryResource, "res1", "LXC 106 running")
	ka.AddFact(FactCategoryStorage, "stor1", "PBS available, 42% used")
	ka.AddFact(FactCategoryAlert, "alert1", "2 active alerts")
	ka.AddFact(FactCategoryExec, "exec1", "exit=0, service active")
	ka.AddFact(FactCategoryFinding, "find1", "warning: high CPU on vm101")

	rendered := ka.Render()

	// Verify category headers appear
	assert.Contains(t, rendered, "Resources:")
	assert.Contains(t, rendered, "Storage:")
	assert.Contains(t, rendered, "Alerts:")
	assert.Contains(t, rendered, "Exec:")
	assert.Contains(t, rendered, "Findings:")

	// Verify values appear
	assert.Contains(t, rendered, "LXC 106 running")
	assert.Contains(t, rendered, "PBS available, 42% used")
	assert.Contains(t, rendered, "2 active alerts")
	assert.Contains(t, rendered, "exit=0, service active")
	assert.Contains(t, rendered, "warning: high CPU on vm101")
}

func TestKnowledgeAccumulator_AlertCategoryOrder(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	ka.AddFact(FactCategoryAlert, "a1", "alert data")
	ka.AddFact(FactCategoryStorage, "s1", "storage data")

	rendered := ka.Render()
	// Alerts should appear after Storage in the render order
	storIdx := strings.Index(rendered, "Storage:")
	alertIdx := strings.Index(rendered, "Alerts:")
	assert.Greater(t, alertIdx, storIdx, "Alerts should come after Storage")
}

func TestKnowledgeAccumulator_RenderOrder(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	// Add in non-category order
	ka.AddFact(FactCategoryFinding, "f1", "finding")
	ka.AddFact(FactCategoryResource, "r1", "resource")

	rendered := ka.Render()
	// Resources should appear before Findings in the rendered output
	resIdx := strings.Index(rendered, "Resources:")
	findIdx := strings.Index(rendered, "Findings:")
	assert.Greater(t, findIdx, resIdx, "Resources should come before Findings")
}

func TestKnowledgeAccumulator_Lookup(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	ka.AddFact(FactCategoryDiscovery, "discovery:delly:106", "service=Postfix, hostname=patrol-signal-test")

	val, found := ka.Lookup("discovery:delly:106")
	assert.True(t, found)
	assert.Equal(t, "service=Postfix, hostname=patrol-signal-test", val)

	val, found = ka.Lookup("discovery:minipc:200")
	assert.False(t, found)
	assert.Equal(t, "", val)
}

func TestKnowledgeAccumulator_SetTurn(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	ka.SetTurn(3)
	ka.AddFact(FactCategoryResource, "k1", "val1")

	fact := ka.facts["k1"]
	require.NotNil(t, fact)
	assert.Equal(t, 3, fact.Turn)
}

func TestKnowledgeAccumulator_FactSummaryForTool(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	// Add facts associated with a tool call
	ka.AddFactForTool("tc-abc", FactCategoryStorage, "storage:delly:pbs-minipc", "PBS, available, 42.7% used, 573GB free")
	ka.AddFactForTool("tc-abc", FactCategoryStorage, "storage:delly:local-lvm", "dir, available, 80% used, 20GB free")

	// Add a fact for a different tool call
	ka.AddFactForTool("tc-xyz", FactCategoryResource, "lxc:delly:106:status", "running, Postfix")

	// FactSummaryForTool should return "key = value" pairs joined with semicolons
	summary := ka.FactSummaryForTool("tc-abc")
	assert.Contains(t, summary, "storage:delly:pbs-minipc = PBS")
	assert.Contains(t, summary, "42.7% used")
	assert.Contains(t, summary, "storage:delly:local-lvm = dir")
	assert.Contains(t, summary, "80% used")
	assert.Contains(t, summary, "; ") // Joined with semicolons

	// tc-xyz should only have its own fact
	summary2 := ka.FactSummaryForTool("tc-xyz")
	assert.Contains(t, summary2, "lxc:delly:106:status = running, Postfix")
	assert.NotContains(t, summary2, "PBS")

	// Unknown tool ID returns empty
	assert.Equal(t, "", ka.FactSummaryForTool("tc-unknown"))
}

func TestKnowledgeAccumulator_AddFactForTool_EmptyToolID(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	// Empty tool ID should still add the fact (just not track the association)
	ka.AddFactForTool("", FactCategoryResource, "key1", "value1")
	assert.Equal(t, 1, ka.Len())
	assert.Equal(t, "", ka.FactSummaryForTool(""))
}

func TestKnowledgeAccumulator_MarkerFactsHiddenFromRender(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	ka.AddFact(FactCategoryStorage, "storage:pools:queried", "11 pools extracted")
	ka.AddFact(FactCategoryStorage, "storage:delly:local-lvm", "dir, 80% used")

	rendered := ka.Render()
	assert.NotContains(t, rendered, "storage:pools:queried")
	assert.NotContains(t, rendered, "11 pools extracted")
	assert.Contains(t, rendered, "[storage:delly:local-lvm] dir, 80% used")
}

func TestKnowledgeAccumulator_MarkerFactsDontCountChars(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	ka.AddFact(FactCategoryStorage, "storage:pools:queried", "11 pools extracted")
	assert.Equal(t, 0, ka.TotalChars(), "marker fact chars should not count toward budget")

	ka.AddFact(FactCategoryStorage, "storage:delly:local-lvm", "dir, 80% used")
	assert.Equal(t, len("dir, 80% used"), ka.TotalChars(), "only non-marker chars should count")
}

func TestKnowledgeAccumulator_MarkerFactsStillLookupable(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	ka.AddFact(FactCategoryStorage, "storage:pools:queried", "11 pools extracted")

	val, found := ka.Lookup("storage:pools:queried")
	assert.True(t, found, "marker facts should still be lookupable by the gate")
	assert.Equal(t, "11 pools extracted", val)
}

func TestRelatedFacts_StoragePools(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	ka.AddFact(FactCategoryStorage, "storage:pools:queried", "3 pools extracted")
	ka.AddFact(FactCategoryStorage, "storage:delly:local-lvm", "dir, 80% used")
	ka.AddFact(FactCategoryStorage, "storage:delly:pbs-minipc", "PBS, 42% used")
	ka.AddFact(FactCategoryStorage, "storage:minipc:local", "dir, 50% used")

	related := ka.RelatedFacts("storage:")
	assert.Contains(t, related, "storage:delly:local-lvm = dir, 80% used")
	assert.Contains(t, related, "storage:delly:pbs-minipc = PBS, 42% used")
	assert.Contains(t, related, "storage:minipc:local = dir, 50% used")
	assert.NotContains(t, related, "pools:queried", "marker should be excluded")
}

func TestRelatedFacts_ExcludesMarkers(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	ka.AddFact(FactCategoryStorage, "disk_health:queried", "2 hosts extracted")
	ka.AddFact(FactCategoryStorage, "disk_health:delly", "2 disks all PASSED")

	related := ka.RelatedFacts("disk_health:")
	assert.Contains(t, related, "disk_health:delly = 2 disks all PASSED")
	assert.NotContains(t, related, "queried")
}

func TestRelatedFacts_EmptyPrefix(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	ka.AddFact(FactCategoryStorage, "storage:delly:local", "dir, 80% used")

	related := ka.RelatedFacts("nonexistent:")
	assert.Equal(t, "", related)
}

func keyForIndex(i int) string {
	return strings.Repeat("k", i+1)
}

func valForIndex(i int) string {
	return strings.Repeat("v", i+1)
}
