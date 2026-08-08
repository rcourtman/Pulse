package monitoring

// Business-scale estate thresholds. These are the single classification
// definition used by the session capability surface in internal/api and the
// outbound telemetry snapshot. They are product segmentation constants, not
// evidence of conversion or owner approval for a commercial surface.
const (
	BusinessEstateMinPVENodes    = 5
	BusinessEstateMinDockerHosts = 10
	BusinessEstateMinVMwareHosts = 3
)

// BusinessScaleEstateCounts reports whether the given counts cross any
// business-scale estate threshold.
func BusinessScaleEstateCounts(pveNodes, dockerHosts, vmwareHosts int) bool {
	return pveNodes >= BusinessEstateMinPVENodes ||
		dockerHosts >= BusinessEstateMinDockerHosts ||
		vmwareHosts >= BusinessEstateMinVMwareHosts
}

// BusinessScaleEstate reports whether the install-wide aggregate counts cross
// any business-scale estate threshold.
func (c InstallSnapshotCounts) BusinessScaleEstate() bool {
	return BusinessScaleEstateCounts(c.PVENodes, c.DockerHosts, c.VMwareHosts)
}
