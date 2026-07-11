package mutationregistry

import (
	"testing"
)

func TestEveryRegisteredMutationHasDisposition(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMigratingInfrastructureMutationsUseActionLifecycle(t *testing.T) {
	for _, entry := range Entries() {
		if entry.Disposition != DispositionLifecycle {
			continue
		}
		if entry.LifecycleExecutor == "" {
			t.Errorf("%s: lifecycle mutation has no executor", entry.ID)
		}
		if entry.Delivery != DeliveryCommittedLifecycle {
			t.Errorf("%s: lifecycle mutation delivery=%q, want %q", entry.ID, entry.Delivery, DeliveryCommittedLifecycle)
		}
	}
}

func TestAdministrativeExceptionsCannotDispatchInfrastructureActions(t *testing.T) {
	for _, entry := range Entries() {
		if entry.Disposition != DispositionAdministrativeException {
			continue
		}
		if entry.LifecycleExecutor != "" {
			t.Errorf("%s: administrative exception must not bind a lifecycle executor", entry.ID)
		}
		if entry.ResourceClass == ResourceCustomerInfrastructure {
			t.Errorf("%s: customer-infrastructure mutation cannot be an administrative exception", entry.ID)
		}
	}
}

func TestRetiredMutationsFailClosed(t *testing.T) {
	for _, entry := range Entries() {
		if entry.Disposition != DispositionRetiredDenied {
			continue
		}
		if entry.Delivery != DeliveryDeniedBeforeTransport {
			t.Errorf("%s: retired mutation delivery=%q, want denied before transport", entry.ID, entry.Delivery)
		}
		if entry.LifecycleExecutor != "" {
			t.Errorf("%s: retired mutation still binds executor %q", entry.ID, entry.LifecycleExecutor)
		}
	}
}

func TestRegistryLookupReturnsClone(t *testing.T) {
	entry, ok := Lookup("assistant.raw-command")
	if !ok {
		t.Fatal("assistant.raw-command missing")
	}
	entry.ResidualOwners[0] = "mutated"
	again, _ := Lookup("assistant.raw-command")
	if again.ResidualOwners[0] == "mutated" {
		t.Fatal("registry exposed mutable generated state")
	}
}

func TestRuntimeCandidateAuditNegativeFixtures(t *testing.T) {
	t.Run("unknown candidate", func(t *testing.T) {
		err := AuditRuntimeCandidates([]RuntimeCandidate{{Surface: "fixture:new-route", MutationID: "fixture.unregistered"}})
		if err == nil {
			t.Fatal("unregistered mutation candidate passed")
		}
	})
	t.Run("alias shadow", func(t *testing.T) {
		err := AuditRuntimeCandidates([]RuntimeCandidate{
			{Surface: "fixture:alias", MutationID: "assistant.raw-command"},
			{Surface: "fixture:alias", MutationID: "legacy.model.run-command"},
		})
		if err == nil {
			t.Fatal("multiply classified alias passed")
		}
	})
	t.Run("transport before authority", func(t *testing.T) {
		err := AuditRuntimeCandidates([]RuntimeCandidate{{
			Surface: "fixture:transport", MutationID: "transport.agent.host-package-update", Transport: true,
		}})
		if err == nil {
			t.Fatal("transport without durable authority passed")
		}
	})
}
