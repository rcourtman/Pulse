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
	assert.Contains(t, rendered, "running, Postfix")
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
	ka.AddFact(FactCategoryExec, "exec1", "exit=0, service active")
	ka.AddFact(FactCategoryFinding, "find1", "warning: high CPU on vm101")

	rendered := ka.Render()

	// Verify category headers appear
	assert.Contains(t, rendered, "Resources:")
	assert.Contains(t, rendered, "Storage:")
	assert.Contains(t, rendered, "Exec:")
	assert.Contains(t, rendered, "Findings:")

	// Verify values appear
	assert.Contains(t, rendered, "LXC 106 running")
	assert.Contains(t, rendered, "PBS available, 42% used")
	assert.Contains(t, rendered, "exit=0, service active")
	assert.Contains(t, rendered, "warning: high CPU on vm101")
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

func keyForIndex(i int) string {
	return strings.Repeat("k", i+1)
}

func valForIndex(i int) string {
	return strings.Repeat("v", i+1)
}
