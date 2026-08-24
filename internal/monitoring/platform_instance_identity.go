package monitoring

// PBSMonitorResourceID is the stable runtime identity used by PBS state,
// alerts, and canonical-resource alias resolution for one configured instance.
func PBSMonitorResourceID(instanceName string) string {
	return "pbs-" + instanceName
}

// PMGMonitorResourceID is the stable runtime identity used by PMG state,
// alerts, and canonical-resource alias resolution for one configured instance.
func PMGMonitorResourceID(instanceName string) string {
	return "pmg-" + instanceName
}
