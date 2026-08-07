package monitoring

// Business-scale estate thresholds. These mirror the segmentation used in the
// 2026-08-07 telemetry read that motivated the commercial-surface revision:
// installs at or above any one of these convert to paid at ~8x the rate of
// smaller estates. This is the single definition; the session capability
// surface in internal/api and the outbound telemetry snapshot both classify
// against it.
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
