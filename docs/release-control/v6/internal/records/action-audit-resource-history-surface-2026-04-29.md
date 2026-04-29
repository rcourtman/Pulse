# Action Audit Resource History Surface

Date: 2026-04-29
Lane: L20 action governance and auditability
Follow-up: `action-governance-auditability-post-rc-hardening`

## Outcome

Resource-detail workflows now have a canonical action-history surface backed by
the unified action audit API. The frontend reads `GET /api/audit/actions`
through `frontend-modern/src/api/actionAudit.ts`, renders resource-scoped
records through `ResourceActionHistory.tsx`, and treats gated or unavailable
audit reads as absent instead of turning ordinary resource inspection into an
upgrade prompt.

## Proof

- `npm --prefix frontend-modern test -- src/api/__tests__/actionAudit.test.ts src/utils/__tests__/actionAuditPresentation.test.ts src/components/Infrastructure/__tests__/ResourceDetailDrawer.history.test.tsx`
- `npm --prefix frontend-modern run type-check`
- `npm --prefix frontend-modern run lint:eslint -- --quiet`
- In-browser resource drawer inspection on the local v6 app confirmed the
  default self-hosted surface still renders the resource drawer without exposing
  action-history upgrade pressure when the action-audit endpoint is unavailable
  or empty.
