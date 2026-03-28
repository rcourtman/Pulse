package api

import (
	"fmt"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type relayMobileRuntimeRouteID string

type relayMobileRuntimeRouteSpec struct {
	id            relayMobileRuntimeRouteID
	method        string
	path          string
	requiredScope string
}

const (
	relayMobileRouteOnboardingQR                 relayMobileRuntimeRouteID = "onboarding-qr"
	relayMobileRouteOnboardingValidate           relayMobileRuntimeRouteID = "onboarding-validate"
	relayMobileRouteOnboardingDeepLink           relayMobileRuntimeRouteID = "onboarding-deep-link"
	relayMobileRoutePatrolFindingsList           relayMobileRuntimeRouteID = "patrol-findings-list"
	relayMobileRouteFindingInvestigation         relayMobileRuntimeRouteID = "finding-investigation"
	relayMobileRouteFindingInvestigationMessages relayMobileRuntimeRouteID = "finding-investigation-messages"
	relayMobileRoutePatrolAcknowledge            relayMobileRuntimeRouteID = "patrol-acknowledge"
	relayMobileRoutePatrolDismiss                relayMobileRuntimeRouteID = "patrol-dismiss"
	relayMobileRoutePatrolSnooze                 relayMobileRuntimeRouteID = "patrol-snooze"
	relayMobileRouteApprovalsList                relayMobileRuntimeRouteID = "approvals-list"
	relayMobileRouteApprovalApprove              relayMobileRuntimeRouteID = "approval-approve"
	relayMobileRouteApprovalDeny                 relayMobileRuntimeRouteID = "approval-deny"
	relayMobileRouteChatSend                     relayMobileRuntimeRouteID = "chat-send"
	relayMobileRouteSessionsList                 relayMobileRuntimeRouteID = "sessions-list"
	relayMobileRouteSessionCreate                relayMobileRuntimeRouteID = "session-create"
	relayMobileRouteSessionMessages              relayMobileRuntimeRouteID = "session-messages"
	relayMobileRouteSessionAbort                 relayMobileRuntimeRouteID = "session-abort"
	relayMobileRouteSessionDelete                relayMobileRuntimeRouteID = "session-delete"
)

var relayMobileRuntimeRouteOrder = []relayMobileRuntimeRouteID{
	relayMobileRouteOnboardingQR,
	relayMobileRouteOnboardingValidate,
	relayMobileRouteOnboardingDeepLink,
	relayMobileRoutePatrolFindingsList,
	relayMobileRouteFindingInvestigation,
	relayMobileRouteFindingInvestigationMessages,
	relayMobileRoutePatrolAcknowledge,
	relayMobileRoutePatrolDismiss,
	relayMobileRoutePatrolSnooze,
	relayMobileRouteApprovalsList,
	relayMobileRouteApprovalApprove,
	relayMobileRouteApprovalDeny,
	relayMobileRouteChatSend,
	relayMobileRouteSessionsList,
	relayMobileRouteSessionCreate,
	relayMobileRouteSessionMessages,
	relayMobileRouteSessionAbort,
	relayMobileRouteSessionDelete,
}

var relayMobileRuntimeRouteSpecs = map[relayMobileRuntimeRouteID]relayMobileRuntimeRouteSpec{
	relayMobileRouteOnboardingQR: {
		id:            relayMobileRouteOnboardingQR,
		method:        http.MethodGet,
		path:          "/api/onboarding/qr",
		requiredScope: config.ScopeSettingsRead,
	},
	relayMobileRouteOnboardingValidate: {
		id:            relayMobileRouteOnboardingValidate,
		method:        http.MethodPost,
		path:          "/api/onboarding/validate",
		requiredScope: config.ScopeSettingsRead,
	},
	relayMobileRouteOnboardingDeepLink: {
		id:            relayMobileRouteOnboardingDeepLink,
		method:        http.MethodGet,
		path:          "/api/onboarding/deep-link",
		requiredScope: config.ScopeSettingsRead,
	},
	relayMobileRoutePatrolFindingsList: {
		id:            relayMobileRoutePatrolFindingsList,
		method:        http.MethodGet,
		path:          "/api/ai/patrol/findings",
		requiredScope: config.ScopeAIExecute,
	},
	relayMobileRouteFindingInvestigation: {
		id:            relayMobileRouteFindingInvestigation,
		method:        http.MethodGet,
		path:          "/api/ai/findings/{finding_id}/investigation",
		requiredScope: config.ScopeAIExecute,
	},
	relayMobileRouteFindingInvestigationMessages: {
		id:            relayMobileRouteFindingInvestigationMessages,
		method:        http.MethodGet,
		path:          "/api/ai/findings/{finding_id}/investigation/messages",
		requiredScope: config.ScopeAIExecute,
	},
	relayMobileRoutePatrolAcknowledge: {
		id:            relayMobileRoutePatrolAcknowledge,
		method:        http.MethodPost,
		path:          "/api/ai/patrol/acknowledge",
		requiredScope: config.ScopeAIExecute,
	},
	relayMobileRoutePatrolDismiss: {
		id:            relayMobileRoutePatrolDismiss,
		method:        http.MethodPost,
		path:          "/api/ai/patrol/dismiss",
		requiredScope: config.ScopeAIExecute,
	},
	relayMobileRoutePatrolSnooze: {
		id:            relayMobileRoutePatrolSnooze,
		method:        http.MethodPost,
		path:          "/api/ai/patrol/snooze",
		requiredScope: config.ScopeAIExecute,
	},
	relayMobileRouteApprovalsList: {
		id:            relayMobileRouteApprovalsList,
		method:        http.MethodGet,
		path:          "/api/ai/approvals",
		requiredScope: config.ScopeAIExecute,
	},
	relayMobileRouteApprovalApprove: {
		id:            relayMobileRouteApprovalApprove,
		method:        http.MethodPost,
		path:          "/api/ai/approvals/{approval_id}/approve",
		requiredScope: config.ScopeAIExecute,
	},
	relayMobileRouteApprovalDeny: {
		id:            relayMobileRouteApprovalDeny,
		method:        http.MethodPost,
		path:          "/api/ai/approvals/{approval_id}/deny",
		requiredScope: config.ScopeAIExecute,
	},
	relayMobileRouteChatSend: {
		id:            relayMobileRouteChatSend,
		method:        http.MethodPost,
		path:          "/api/ai/chat",
		requiredScope: config.ScopeAIChat,
	},
	relayMobileRouteSessionsList: {
		id:            relayMobileRouteSessionsList,
		method:        http.MethodGet,
		path:          "/api/ai/sessions",
		requiredScope: config.ScopeAIChat,
	},
	relayMobileRouteSessionCreate: {
		id:            relayMobileRouteSessionCreate,
		method:        http.MethodPost,
		path:          "/api/ai/sessions",
		requiredScope: config.ScopeAIChat,
	},
	relayMobileRouteSessionMessages: {
		id:            relayMobileRouteSessionMessages,
		method:        http.MethodGet,
		path:          "/api/ai/sessions/{session_id}/messages",
		requiredScope: config.ScopeAIChat,
	},
	relayMobileRouteSessionAbort: {
		id:            relayMobileRouteSessionAbort,
		method:        http.MethodPost,
		path:          "/api/ai/sessions/{session_id}/abort",
		requiredScope: config.ScopeAIChat,
	},
	relayMobileRouteSessionDelete: {
		id:            relayMobileRouteSessionDelete,
		method:        http.MethodDelete,
		path:          "/api/ai/sessions/{session_id}",
		requiredScope: config.ScopeAIChat,
	},
}

func relayMobileRuntimeRouteInventory() []relayMobileRuntimeRouteSpec {
	inventory := make([]relayMobileRuntimeRouteSpec, 0, len(relayMobileRuntimeRouteOrder))
	for _, routeID := range relayMobileRuntimeRouteOrder {
		inventory = append(inventory, relayMobileRuntimeRouteSpecFor(routeID))
	}
	return inventory
}

func relayMobileRuntimeRouteSpecFor(routeID relayMobileRuntimeRouteID) relayMobileRuntimeRouteSpec {
	spec, ok := relayMobileRuntimeRouteSpecs[routeID]
	if !ok {
		panic(fmt.Sprintf("unknown relay mobile runtime route %q", routeID))
	}
	return spec
}

func (spec relayMobileRuntimeRouteSpec) compatibleScopes() []string {
	return []string{config.ScopeRelayMobileAccess, spec.requiredScope}
}

func requireRelayMobileRuntimeRoute(routeID relayMobileRuntimeRouteID, handler http.HandlerFunc) http.HandlerFunc {
	return RequireAnyScope(relayMobileRuntimeRouteSpecFor(routeID).compatibleScopes(), handler)
}

func ensureRelayMobileRuntimeRoute(w http.ResponseWriter, r *http.Request, routeID relayMobileRuntimeRouteID) bool {
	return ensureAnyScope(w, r, relayMobileRuntimeRouteSpecFor(routeID).compatibleScopes()...)
}
