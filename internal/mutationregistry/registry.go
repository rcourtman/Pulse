// Package mutationregistry owns the closed product-wide classification of
// runtime-reachable infrastructure mutations. Runtime surfaces may execute a
// mutation only when this registry classifies it as canonical lifecycle work;
// retired entries must fail before transport, and administrative exceptions
// must not target customer infrastructure.
package mutationregistry

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type Disposition string
type Origin string
type ResourceClass string
type ApprovalFloor string
type DeliveryClass string
type VerificationClass string
type RollbackClass string

const (
	DispositionLifecycle               Disposition = "lifecycle"
	DispositionAdministrativeException Disposition = "administrative_exception"
	DispositionRetiredDenied           Disposition = "retired_denied"

	OriginAPI       Origin = "api"
	OriginJob       Origin = "job"
	OriginLegacy    Origin = "legacy"
	OriginModel     Origin = "model"
	OriginTransport Origin = "transport"

	ResourceCustomerInfrastructure ResourceClass = "customer_infrastructure"
	ResourcePulseAdministration    ResourceClass = "pulse_administration"

	ApprovalNone   ApprovalFloor = "none"
	ApprovalAdmin  ApprovalFloor = "admin"
	ApprovalPolicy ApprovalFloor = "policy_or_admin"

	DeliveryCommittedLifecycle    DeliveryClass = "committed_lifecycle_before_transport"
	DeliveryAdministrative        DeliveryClass = "administrative_transaction"
	DeliveryDeniedBeforeTransport DeliveryClass = "denied_before_transport"

	VerificationRequired       VerificationClass = "required"
	VerificationAdministrative VerificationClass = "administrative"
	VerificationDenied         VerificationClass = "denied"

	RollbackUnsupported RollbackClass = "unsupported"
	RollbackTask10      RollbackClass = "task_10_truth_and_compensation"
	RollbackDenied      RollbackClass = "denied"
)

// Entry is generated from manifest.json. Capability is the canonical resource
// capability where one exists; Entrypoint names the live or compatibility
// surface whose disposition is enforced.
type Entry struct {
	ID                string            `json:"id"`
	Origin            Origin            `json:"origin"`
	ResourceClass     ResourceClass     `json:"resource_class"`
	ResourceKind      string            `json:"resource_kind"`
	Capability        string            `json:"capability"`
	Entrypoint        string            `json:"entrypoint"`
	Disposition       Disposition       `json:"disposition"`
	LifecycleExecutor string            `json:"lifecycle_executor,omitempty"`
	Approval          ApprovalFloor     `json:"approval_floor"`
	Delivery          DeliveryClass     `json:"delivery"`
	Verification      VerificationClass `json:"verification"`
	Rollback          RollbackClass     `json:"rollback"`
	ResidualOwners    []string          `json:"residual_owners,omitempty"`
}

func Entries() []Entry {
	out := make([]Entry, len(generatedEntries))
	for i := range generatedEntries {
		out[i] = cloneEntry(generatedEntries[i])
	}
	return out
}

func Lookup(id string) (Entry, bool) {
	entry, ok := generatedByID[strings.TrimSpace(id)]
	return cloneEntry(entry), ok
}

// LookupModelInvocation maps every model-reachable infrastructure mutation to
// its closed registry identity. The mapping is intentionally string-based so
// the registry remains below provider/tool packages and cannot acquire a
// dependency cycle. Unknown invocations are not treated as safe by callers;
// their existing fail-closed classifier still applies.
func LookupModelInvocation(toolName string, args map[string]interface{}) (Entry, bool) {
	toolName = strings.TrimSpace(toolName)
	stringArg := func(key string) string {
		value, _ := args[key].(string)
		return strings.ToLower(strings.TrimSpace(value))
	}
	id := ""
	switch toolName {
	case "pulse_control":
		switch stringArg("type") {
		case "resource":
			id = "assistant.resource-action"
		case "guest":
			id = "assistant.guest.control"
		case "command":
			id = "assistant.raw-command"
		}
	case "pulse_docker":
		switch stringArg("action") {
		case "control":
			id = "assistant.docker.control"
		case "update":
			id = "assistant.docker.update"
		}
	case "pulse_kubernetes":
		switch stringArg("type") {
		case "scale", "restart", "exec":
			id = "assistant.kubernetes." + stringArg("type")
		case "delete_pod":
			id = "assistant.kubernetes.delete-pod"
		}
	case "pulse_file_edit":
		switch stringArg("action") {
		case "append", "write":
			id = "assistant.file." + stringArg("action")
		}
	case "run_command", "pulse_run_command":
		id = "legacy.model.run-command"
	}
	if id == "" {
		return Entry{}, false
	}
	return Lookup(id)
}

func cloneEntry(entry Entry) Entry {
	entry.ResidualOwners = append([]string(nil), entry.ResidualOwners...)
	return entry
}

func Validate() error {
	if len(generatedEntries) == 0 {
		return errors.New("mutation registry is empty")
	}
	seen := make(map[string]struct{}, len(generatedEntries))
	for i, entry := range generatedEntries {
		if strings.TrimSpace(entry.ID) == "" || strings.TrimSpace(entry.Entrypoint) == "" {
			return fmt.Errorf("entry %d has an empty id or entrypoint", i)
		}
		if _, duplicate := seen[entry.ID]; duplicate {
			return fmt.Errorf("duplicate mutation id %q", entry.ID)
		}
		seen[entry.ID] = struct{}{}
		switch entry.Disposition {
		case DispositionLifecycle:
			if entry.ResourceClass != ResourceCustomerInfrastructure || entry.LifecycleExecutor == "" || entry.Delivery != DeliveryCommittedLifecycle {
				return fmt.Errorf("lifecycle mutation %q has an incomplete canonical binding", entry.ID)
			}
		case DispositionAdministrativeException:
			if entry.ResourceClass != ResourcePulseAdministration || entry.LifecycleExecutor != "" || entry.Delivery != DeliveryAdministrative {
				return fmt.Errorf("administrative exception %q crosses the infrastructure boundary", entry.ID)
			}
		case DispositionRetiredDenied:
			if entry.LifecycleExecutor != "" || entry.Delivery != DeliveryDeniedBeforeTransport || entry.Verification != VerificationDenied || entry.Rollback != RollbackDenied {
				return fmt.Errorf("retired mutation %q is not fully denied", entry.ID)
			}
		default:
			return fmt.Errorf("mutation %q has unknown disposition %q", entry.ID, entry.Disposition)
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	if !sort.StringsAreSorted(ids) {
		sort.Strings(ids)
	}
	return nil
}

func init() {
	if err := Validate(); err != nil {
		panic(err)
	}
}
