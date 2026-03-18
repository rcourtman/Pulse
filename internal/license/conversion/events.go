package conversion

import pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"

const (
	EventPaywallViewed               = pkglicensing.EventPaywallViewed
	EventTrialStarted                = pkglicensing.EventTrialStarted
	EventLicenseActivated            = pkglicensing.EventLicenseActivated
	EventLicenseActivationFailed     = pkglicensing.EventLicenseActivationFailed
	EventUpgradeClicked              = pkglicensing.EventUpgradeClicked
	EventCheckoutStarted             = pkglicensing.EventCheckoutStarted
	EventCheckoutCompleted           = pkglicensing.EventCheckoutCompleted
	EventLimitWarningShown           = pkglicensing.EventLimitWarningShown
	EventLimitBlocked                = pkglicensing.EventLimitBlocked
	EventAgentInstallTokenGenerated  = pkglicensing.EventAgentInstallTokenGenerated
	EventAgentInstallCommandCopied   = pkglicensing.EventAgentInstallCommandCopied
	EventAgentInstallProfileSelected = pkglicensing.EventAgentInstallProfileSelected
	EventAgentFirstConnected         = pkglicensing.EventAgentFirstConnected
)

type ConversionEvent = pkglicensing.ConversionEvent
