package unified

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func setUnexportedField(t *testing.T, target interface{}, fieldName string, value interface{}) {
	t.Helper()

	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Ptr {
		t.Fatalf("target must be a pointer")
	}
	field := val.Elem().FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("field %s not found", fieldName)
	}

	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}

func getUnexportedField(t *testing.T, target interface{}, fieldName string) reflect.Value {
	t.Helper()

	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Ptr {
		t.Fatalf("target must be a pointer")
	}
	field := val.Elem().FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("field %s not found", fieldName)
	}

	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
}

func TestAlertManagerAdapter_NilManager(t *testing.T) {
	adapter := NewAlertManagerAdapter(nil)
	if adapter.GetActiveAlerts() != nil {
		t.Fatalf("expected nil alerts for nil manager")
	}
	if adapter.GetAlert("missing") != nil {
		t.Fatalf("expected nil alert for nil manager")
	}

	adapter.SetAlertCallback(nil)
	adapter.SetResolvedCallback(nil)
}

func TestAlertManagerAdapter_WithManagerAndCallbacks(t *testing.T) {
	manager := alerts.NewManager()
	alert := &alerts.Alert{
		ID:           "alert-1",
		Type:         "cpu",
		Level:        alerts.AlertLevelCritical,
		ResourceID:   "vm-100",
		ResourceName: "web-1",
		Node:         "node-1",
		Message:      "high cpu",
		Value:        92.5,
		Threshold:    80,
		StartTime:    time.Now().Add(-time.Minute),
		LastSeen:     time.Now(),
		Metadata:     map[string]interface{}{"resourceType": "node"},
	}

	activeAlerts := map[string]*alerts.Alert{
		alert.ID: alert,
	}
	setUnexportedField(t, manager, "activeAlerts", activeAlerts)

	adapter := NewAlertManagerAdapter(manager)
	active := adapter.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("expected 1 active alert, got %d", len(active))
	}
	if active[0].GetAlertID() != alert.ID {
		t.Fatalf("expected alert ID %s", alert.ID)
	}
	if active[0].GetAlertLevel() != string(alert.Level) {
		t.Fatalf("expected alert level %s", alert.Level)
	}

	found := adapter.GetAlert(alert.ID)
	if found == nil || found.GetAlertID() != alert.ID {
		t.Fatalf("expected to find alert %s", alert.ID)
	}
	if adapter.GetAlert("missing") != nil {
		t.Fatalf("expected nil for missing alert")
	}

	alertCh := make(chan string, 1)
	adapter.SetAlertCallback(func(ad AlertAdapter) {
		alertCh <- ad.GetAlertID()
	})
	onAlert := getUnexportedField(t, manager, "onAlert").Interface().(func(alert *alerts.Alert))
	onAlert(alert)
	select {
	case got := <-alertCh:
		if got != alert.ID {
			t.Fatalf("expected alert callback for %s, got %s", alert.ID, got)
		}
	default:
		t.Fatalf("expected alert callback to fire")
	}

	resolvedCh := make(chan string, 1)
	adapter.SetResolvedCallback(func(alertID string) {
		resolvedCh <- alertID
	})
	onResolved := getUnexportedField(t, manager, "onResolved").Interface().(func(alertID string))
	onResolved(alert.ID)
	select {
	case got := <-resolvedCh:
		if got != alert.ID {
			t.Fatalf("expected resolved callback for %s, got %s", alert.ID, got)
		}
	default:
		t.Fatalf("expected resolved callback to fire")
	}
}

func TestAlertWrapper_NilAlert(t *testing.T) {
	wrapper := &alertWrapper{}
	if wrapper.GetAlertID() != "" {
		t.Fatalf("expected empty ID")
	}
	if wrapper.GetAlertType() != "" {
		t.Fatalf("expected empty type")
	}
	if wrapper.GetAlertLevel() != "" {
		t.Fatalf("expected empty level")
	}
	if wrapper.GetResourceID() != "" {
		t.Fatalf("expected empty resource id")
	}
	if wrapper.GetResourceName() != "" {
		t.Fatalf("expected empty resource name")
	}
	if wrapper.GetNode() != "" {
		t.Fatalf("expected empty node")
	}
	if wrapper.GetMessage() != "" {
		t.Fatalf("expected empty message")
	}
	if wrapper.GetValue() != 0 {
		t.Fatalf("expected zero value")
	}
	if wrapper.GetThreshold() != 0 {
		t.Fatalf("expected zero threshold")
	}
	if !wrapper.GetStartTime().IsZero() {
		t.Fatalf("expected zero start time")
	}
	if !wrapper.GetLastSeen().IsZero() {
		t.Fatalf("expected zero last seen time")
	}
	if wrapper.GetMetadata() != nil {
		t.Fatalf("expected nil metadata")
	}
}

func TestAlertAdapter_ResourceTypeFromMetadata(t *testing.T) {
	cases := []struct {
		name         string
		resourceType string
	}{
		{name: "vm", resourceType: "VM"},
		{name: "container", resourceType: "Container"},
		{name: "node", resourceType: "Node"},
		{name: "host", resourceType: "Host"},
		{name: "docker_container", resourceType: "Docker Container"},
		{name: "docker_host", resourceType: "DockerHost"},
		{name: "docker_service", resourceType: "Docker Service"},
		{name: "pbs", resourceType: "PBS"},
		{name: "storage", resourceType: "Storage"},
		{name: "pmg", resourceType: "PMG"},
		{name: "k8s", resourceType: "K8s"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrapper := &alertWrapper{
				alert: &alerts.Alert{
					ID:       "alert-1",
					Type:     "cpu",
					Metadata: map[string]interface{}{"resourceType": tc.resourceType},
				},
			}

			metadata := wrapper.GetMetadata()
			if metadata == nil {
				t.Fatalf("expected metadata map")
			}
			gotType, ok := metadata["resourceType"].(string)
			if !ok {
				t.Fatalf("expected resourceType string in metadata")
			}
			if gotType != tc.resourceType {
				t.Fatalf("metadata resourceType mismatch: got %q want %q", gotType, tc.resourceType)
			}

			determined := determineResourceType(wrapper.GetAlertType(), metadata)
			if determined != tc.resourceType {
				t.Fatalf("determineResourceType mismatch: got %q want %q", determined, tc.resourceType)
			}
		})
	}
}
