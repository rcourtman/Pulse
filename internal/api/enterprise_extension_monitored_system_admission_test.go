package api

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

func TestGetResolveMonitoredSystemAdmissionPolicy_DefaultNil(t *testing.T) {
	SetResolveMonitoredSystemAdmissionPolicy(nil)
	t.Cleanup(func() { SetResolveMonitoredSystemAdmissionPolicy(nil) })

	if getResolveMonitoredSystemAdmissionPolicy() != nil {
		t.Fatal("expected nil when no monitored-system admission policy hook is registered")
	}
}

func TestGetResolveMonitoredSystemAdmissionPolicy_RoundTripsHook(t *testing.T) {
	SetResolveMonitoredSystemAdmissionPolicy(nil)
	t.Cleanup(func() { SetResolveMonitoredSystemAdmissionPolicy(nil) })

	SetResolveMonitoredSystemAdmissionPolicy(func(_ context.Context, input extensions.MonitoredSystemAdmissionInput) extensions.MonitoredSystemAdmissionDecision {
		return extensions.MonitoredSystemAdmissionDecision{
			Current:                input.Current,
			Additional:             input.Additional,
			Limit:                  input.Limit,
			UsageAvailable:         input.UsageAvailable,
			UsageUnavailableReason: input.UsageUnavailableReason,
			Exceeded:               input.UsageAvailable && input.Additional > 0 && input.Limit > 0 && input.Current+input.Additional > input.Limit,
		}
	})

	hook := getResolveMonitoredSystemAdmissionPolicy()
	if hook == nil {
		t.Fatal("expected monitored-system admission policy hook to be returned")
	}

	decision := hook(context.Background(), extensions.MonitoredSystemAdmissionInput{
		Current:                5,
		Additional:             1,
		Limit:                  5,
		UsageAvailable:         true,
		UsageUnavailableReason: "",
	})
	if !decision.Exceeded {
		t.Fatalf("expected hook result to preserve exceeded state, got %+v", decision)
	}
	if decision.Current != 5 || decision.Additional != 1 || decision.Limit != 5 {
		t.Fatalf("expected hook result to preserve admission values, got %+v", decision)
	}
}
