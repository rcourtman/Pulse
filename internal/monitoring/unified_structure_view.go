package monitoring

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// cachedMockStructureView returns the mock view the monitor has cached when it
// was built for the fixture's current structure revision and operator link
// list, however many metric ticks ago. The second result is false when there is
// no such view, and the first is then not for use.
func (m *Monitor) cachedMockStructureView() (monitorUnifiedStateView, bool) {
	structure := mock.FixtureStructureRevision()
	links := m.resourceStoreManualLinks()

	m.mockUnifiedViewMu.Lock()
	defer m.mockUnifiedViewMu.Unlock()
	reusable := m.mockUnifiedViewValid &&
		m.mockUnifiedViewStructure == structure &&
		sameManualLinks(m.mockUnifiedViewLinks, links)
	return m.mockUnifiedView, reusable
}

// currentStructureUnifiedStateView is currentUnifiedStateView for a caller that
// reads what the estate lists and how it connects: IDs, names, types, source
// mappings, parents, operator links and the metrics targets those yield. In
// mock mode that answers from the cached view for as long as the fixture
// structure and the link list are unchanged. The view is keyed on the data
// version otherwise, which advances on every metric tick, so a caller that
// needs only structure but asked for the data-version view built the whole
// estate once per tick: about half a second of CPU on an idle core, and on a
// starved process a build longer than the tick, so every call missed. A
// caller must not read what a tick moves (status, metric values, timestamps,
// sensors); the view it gets can be any number of ticks old. Outside mock mode
// this is currentUnifiedStateView.
func (m *Monitor) currentStructureUnifiedStateView() monitorUnifiedStateView {
	if m == nil {
		return monitorUnifiedStateView{}
	}
	if mock.IsMockEnabled() {
		if view, ok := m.cachedMockStructureView(); ok {
			return view
		}
	}
	return m.currentUnifiedStateView()
}

// GetUnifiedStructureReadState is GetUnifiedReadStateOrSnapshot for a caller
// that reads identity and topology only (see currentStructureUnifiedStateView):
// in mock mode it can be several metric ticks old.
func (m *Monitor) GetUnifiedStructureReadState() unifiedresources.ReadState {
	if m == nil {
		return nil
	}
	if !mock.IsMockEnabled() {
		return m.GetUnifiedReadStateOrSnapshot()
	}
	return m.currentStructureUnifiedStateView().readState
}

// currentModeStructureReadState is currentModeReadState for a caller that reads
// identity and topology only.
func (m *Monitor) currentModeStructureReadState() unifiedresources.ReadState {
	if m == nil {
		return nil
	}
	if mock.IsMockEnabled() {
		return m.currentStructureUnifiedStateView().readState
	}
	return m.currentModeReadState()
}

// structureNodes is NodesSnapshot for a caller that reads the nodes' identity
// and links only (ID, instance, linked agent).
func (m *Monitor) structureNodes() []models.Node {
	return nodesSnapshotFromReadState(m.GetUnifiedStructureReadState())
}
