package conversion

import pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"

const (
	EventPricingViewed               = pkglicensing.EventPricingViewed
	EventPaywallViewed               = pkglicensing.EventPaywallViewed
	EventTrialStarted                = pkglicensing.EventTrialStarted
	EventLicenseActivated            = pkglicensing.EventLicenseActivated
	EventLicenseActivationFailed     = pkglicensing.EventLicenseActivationFailed
	EventUpgradeClicked              = pkglicensing.EventUpgradeClicked
	EventCheckoutClicked             = pkglicensing.EventCheckoutClicked
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
