package mock

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// GenerateAlertHistory generates historical alert data for testing
func GenerateAlertHistory(nodes []models.Node, vms []models.VM, containers []models.Container) []models.Alert {
	var history []models.Alert

	// Alert types and messages
	alertTypes := []struct {
		alertType string
		level     string
		messages  []string
	}{
		{
			alertType: "threshold",
			level:     "warning",
			messages: []string{
				"CPU usage exceeded %d%%",
				"Memory usage exceeded %d%%",
				"Disk usage exceeded %d%%",
			},
		},
		{
			alertType: "threshold",
			level:     "critical",
			messages: []string{
				"CPU usage critical at %d%%",
				"Memory exhausted at %d%%",
				"Disk space critical at %d%%",
			},
		},
		{
			alertType: "connectivity",
			level:     "error",
			messages: []string{
				"Node offline",
				"Connection lost",
				"Network timeout",
			},
		},
		{
			alertType: "backup",
			level:     "warning",
			messages: []string{
				"Backup failed",
				"Backup verification failed",
				"Backup storage low",
			},
		},
		{
			alertType: "system",
			level:     "info",
			messages: []string{
				"System update available",
				"Certificate expiring soon",
				"Scheduled maintenance reminder",
			},
		},
	}

	// Generate alerts for the past 90 days with more consistent distribution
	now := time.Now()
	for days := 90; days >= 0; days-- {
		// Generate 2-15 alerts per day for more realistic history
		numAlerts := rand.Intn(14) + 2

		for i := 0; i < numAlerts; i++ {
			// Pick random alert type
			alertType := alertTypes[rand.Intn(len(alertTypes))]

			// Pick random resource
			var resourceName, resourceID, node string
			resourceType := rand.Intn(3)

			switch resourceType {
			case 0: // Node alert
				if len(nodes) > 0 {
					selectedNode := nodes[rand.Intn(len(nodes))]
					resourceName = selectedNode.Name
					resourceID = selectedNode.ID
					node = selectedNode.Name
				}
			case 1: // VM alert
				if len(vms) > 0 {
					vm := vms[rand.Intn(len(vms))]
					resourceName = vm.Name
					resourceID = vm.ID
					node = vm.Node
				}
			case 2: // Container alert
				if len(containers) > 0 {
					selectedContainer := containers[rand.Intn(len(containers))]
					resourceName = selectedContainer.Name
					resourceID = selectedContainer.ID
					node = selectedContainer.Node
				}
			}

			// Random time during that day
			hours := rand.Intn(24)
			minutes := rand.Intn(60)
			seconds := rand.Intn(60)

			startTime := now.AddDate(0, 0, -days).
				Truncate(24 * time.Hour).
				Add(time.Duration(hours) * time.Hour).
				Add(time.Duration(minutes) * time.Minute).
				Add(time.Duration(seconds) * time.Second)

			// Alert duration (resolved after 1 minute to 4 hours) - for display purposes

			// Pick random message and format it
			msg := alertType.messages[rand.Intn(len(alertType.messages))]
			if alertType.alertType == "threshold" {
				value := rand.Intn(30) + 70 // 70-99%
				msg = fmt.Sprintf(msg, value)
			}

			alert := models.Alert{
				ID:           fmt.Sprintf("hist-%d-%d-%d", days, i, rand.Intn(10000)),
				Type:         alertType.alertType,
				Level:        alertType.level,
				ResourceID:   resourceID,
				ResourceName: resourceName,
				Node:         node,
				Message:      msg,
				StartTime:    startTime,
				Acknowledged: false, // Historical alerts are not acknowledged
			}

			// Add threshold values for threshold alerts
			if alertType.alertType == "threshold" {
				value := float64(rand.Intn(30) + 70)
				threshold := float64(rand.Intn(20) + 60)
				alert.Value = value
				alert.Threshold = threshold
			}

			history = append(history, alert)
		}
	}

	// Add some recent unresolved alerts (last 2 hours)
	for i := 0; i < 3; i++ {
		alertType := alertTypes[rand.Intn(len(alertTypes))]

		var resourceName, resourceID, node string
		if len(nodes) > 0 {
			selectedNode := nodes[rand.Intn(len(nodes))]
			resourceName = selectedNode.Name
			resourceID = selectedNode.ID
			node = selectedNode.Name
		}

		startTime := now.Add(-time.Duration(rand.Intn(120)) * time.Minute)

		msg := alertType.messages[rand.Intn(len(alertType.messages))]
		if alertType.alertType == "threshold" {
			value := rand.Intn(30) + 70
			msg = fmt.Sprintf(msg, value)
		}

		alert := models.Alert{
			ID:           fmt.Sprintf("active-%d-%d", i, rand.Intn(10000)),
			Type:         alertType.alertType,
			Level:        alertType.level,
			ResourceID:   resourceID,
			ResourceName: resourceName,
			Node:         node,
			Message:      msg,
			StartTime:    startTime,
			Acknowledged: false,
		}

		if alertType.alertType == "threshold" {
			value := float64(rand.Intn(30) + 70)
			threshold := float64(rand.Intn(20) + 60)
			alert.Value = value
			alert.Threshold = threshold
		}

		history = append(history, alert)
	}

	return history
}
