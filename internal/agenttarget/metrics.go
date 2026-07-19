package agenttarget

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	destinationConfigured = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pulse_agent_destination_configured",
		Help: "Configured Pulse report destinations by module and authority role.",
	}, []string{"module", "destination", "role"})
	destinationDeliveryUp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pulse_agent_destination_delivery_up",
		Help: "Whether the most recent report delivery to a Pulse destination succeeded.",
	}, []string{"module", "destination", "role"})
)

func MarkConfigured(module, destination, role string) {
	destinationConfigured.WithLabelValues(module, destination, role).Set(1)
}

func MarkDelivery(module, destination, role string, success bool) {
	value := 0.0
	if success {
		value = 1
	}
	destinationDeliveryUp.WithLabelValues(module, destination, role).Set(value)
}
