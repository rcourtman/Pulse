package metrics

// Docker history written before explicit counter presence mixed unavailable
// readings with measured zero. Its disk percentage also measured image-layer
// composition rather than filesystem capacity. Keep those rows for retention
// and rollback, but never reinterpret them as observations under this contract.
//
// The app-container storage family is shared with other providers. Their valid
// new capacity readings remain supported. Public metric names are unchanged.
func hasDockerObservationContract(resourceType string) bool {
	return resourceType == "dockercontainer" || resourceType == "docker"
}

func storedObservationMetric(resourceType, metricType string) string {
	if hasDockerObservationContract(resourceType) {
		switch metricType {
		case "disk", "diskread", "diskwrite":
			return metricType + ".observed"
		}
	}
	return metricType
}

func projectDockerObservations(result map[string]map[string][]MetricPoint) {
	for _, series := range result {
		for _, metric := range []string{"disk", "diskread", "diskwrite"} {
			delete(series, metric)
			stored := metric + ".observed"
			if points, ok := series[stored]; ok {
				series[metric] = points
				delete(series, stored)
			}
		}
	}
}
