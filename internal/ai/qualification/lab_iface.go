package qualification

import "context"

// Lab is what a live run needs from a lab driver: prepare the scenario's
// resources, inject and revert its faults, observe its predicates and leave
// the lab as it found it. DockerLab and ProxmoxLab implement it.
type Lab interface {
	Prepare(ctx context.Context, manifest Manifest, runID string) (*PreparedLab, error)
	ApplyFault(ctx context.Context, manifest Manifest, lab *PreparedLab, fault FaultSpec) error
	RevertFault(ctx context.Context, manifest Manifest, lab *PreparedLab, fault FaultSpec) error
	Observe(ctx context.Context, manifest Manifest, lab *PreparedLab, predicates []Predicate) ([]PredicateObservation, error)
	Cleanup(ctx context.Context, manifest Manifest, lab *PreparedLab) CleanupResult
	// Safety oracles observe the same prepared resources as injection and
	// teardown. In particular, Proxmox directory storage need not be local.
	runInventory(ctx context.Context, lab *PreparedLab) (DockerInventory, error)
	targetLabel() string
}

var (
	_ Lab = (*DockerLab)(nil)
	_ Lab = (*ProxmoxLab)(nil)
)

func (l *DockerLab) targetLabel() string { return dockerTargetLabel(l.target) }

func (l *DockerLab) runInventory(ctx context.Context, _ *PreparedLab) (DockerInventory, error) {
	return l.inventory(ctx)
}
