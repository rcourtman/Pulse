package api

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

func TestNewAlertMonitorWrapper_Nil(t *testing.T) {
	if NewAlertMonitorWrapper(nil) != nil {
		t.Fatalf("expected nil wrapper for nil monitor")
	}
}

func TestNewNotificationMonitorWrapper_Nil(t *testing.T) {
	if NewNotificationMonitorWrapper(nil) != nil {
		t.Fatalf("expected nil wrapper for nil monitor")
	}
}

func TestAlertMonitorWrapper_Delegates(t *testing.T) {
	monitor := &monitoring.Monitor{}
	state := models.NewState()
	alertManager := &alerts.Manager{}
	incidentStore := &memory.IncidentStore{}
	notificationMgr := &notifications.NotificationManager{}
	configPersist := &config.ConfigPersistence{}

	setUnexportedField(t, monitor, "state", state)
	setUnexportedField(t, monitor, "alertManager", alertManager)
	setUnexportedField(t, monitor, "incidentStore", incidentStore)
	setUnexportedField(t, monitor, "notificationMgr", notificationMgr)
	setUnexportedField(t, monitor, "configPersist", configPersist)

	wrapper := NewAlertMonitorWrapper(monitor)
	if wrapper == nil {
		t.Fatalf("expected wrapper for non-nil monitor")
	}

	if wrapper.GetAlertManager() != alertManager {
		t.Fatalf("unexpected alert manager")
	}
	if wrapper.GetIncidentStore() != incidentStore {
		t.Fatalf("unexpected incident store")
	}
	if wrapper.GetNotificationManager() != notificationMgr {
		t.Fatalf("unexpected notification manager")
	}
	if wrapper.GetConfigPersistence() != configPersist {
		t.Fatalf("unexpected config persistence")
	}

	expected := state.GetSnapshot()
	if got := wrapper.GetState(); !reflect.DeepEqual(got, expected) {
		t.Fatalf("unexpected state snapshot")
	}

	wrapper.SyncAlertState()
	if state.ActiveAlerts == nil {
		t.Fatalf("expected active alerts to be initialized")
	}
}

func TestNotificationMonitorWrapper_Delegates(t *testing.T) {
	monitor := &monitoring.Monitor{}
	state := models.NewState()
	notificationMgr := &notifications.NotificationManager{}
	configPersist := &config.ConfigPersistence{}

	setUnexportedField(t, monitor, "state", state)
	setUnexportedField(t, monitor, "notificationMgr", notificationMgr)
	setUnexportedField(t, monitor, "configPersist", configPersist)

	wrapper := NewNotificationMonitorWrapper(monitor)
	if wrapper == nil {
		t.Fatalf("expected wrapper for non-nil monitor")
	}

	if wrapper.GetNotificationManager() != notificationMgr {
		t.Fatalf("unexpected notification manager")
	}
	if wrapper.GetConfigPersistence() != configPersist {
		t.Fatalf("unexpected config persistence")
	}

	expected := state.GetSnapshot()
	if got := wrapper.GetState(); !reflect.DeepEqual(got, expected) {
		t.Fatalf("unexpected state snapshot")
	}
}
