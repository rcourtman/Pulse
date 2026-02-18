package recovery

import "github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"

// Re-export the recovery model types/constants from a dependency-minimal package so
// other packages (notably internal/mock) can reference the canonical JSON shapes
// without importing the full recovery store/manager (which depends on config).

type Provider = model.Provider

const (
	ProviderProxmoxPVE Provider = model.ProviderProxmoxPVE
	ProviderProxmoxPBS Provider = model.ProviderProxmoxPBS
	ProviderProxmoxPMG Provider = model.ProviderProxmoxPMG
	ProviderKubernetes Provider = model.ProviderKubernetes
	ProviderTrueNAS    Provider = model.ProviderTrueNAS
	ProviderDocker     Provider = model.ProviderDocker
	ProviderHostAgent  Provider = model.ProviderHostAgent
)

type Kind = model.Kind

const (
	KindSnapshot Kind = model.KindSnapshot
	KindBackup   Kind = model.KindBackup
	KindOther    Kind = model.KindOther
)

type Mode = model.Mode

const (
	ModeSnapshot Mode = model.ModeSnapshot
	ModeLocal    Mode = model.ModeLocal
	ModeRemote   Mode = model.ModeRemote
)

type Outcome = model.Outcome

const (
	OutcomeSuccess Outcome = model.OutcomeSuccess
	OutcomeWarning Outcome = model.OutcomeWarning
	OutcomeFailed  Outcome = model.OutcomeFailed
	OutcomeRunning Outcome = model.OutcomeRunning
	OutcomeUnknown Outcome = model.OutcomeUnknown
)

type ExternalRef = model.ExternalRef
type RecoveryPoint = model.RecoveryPoint
type RecoveryPointDisplay = model.RecoveryPointDisplay
type ProtectionRollup = model.ProtectionRollup
type ListPointsOptions = model.ListPointsOptions
type PointsSeriesBucket = model.PointsSeriesBucket
type PointsFacets = model.PointsFacets
