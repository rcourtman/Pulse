# Localized Product Surfaces Alerts Overview - 2026-06-14

## Slice

- Claimed governed slice: `candidate-lane:localized-product-surfaces`.
- Migrated journey: Alerts Overview operator triage, including the Alerts page shell/header/tab labels, activation toggle labels and feedback, overview stat cards, empty/paused states, active-alert acknowledge controls, incident timeline panel and filters, and Pulse Assistant alert investigation handoff briefing.
- First-wave locales: German (`de`) and Spanish (`es`).

## Evidence

- `frontend-modern/src/i18n/messages.ts` now includes explicit German and Spanish catalog entries for the migrated Alerts Overview operator journey.
- `frontend-modern/src/i18n/policy.ts` records Alerts Overview non-translation tokens and marks the migrated alert keys as a first-wave localized journey.
- `frontend-modern/src/utils/alertOverviewPresentation.ts`, `alertActivationPresentation.ts`, and `alertTabsPresentation.ts` now route user-visible Alerts Overview, activation, tab, timeline, and acknowledgement copy through the canonical i18n layer.
- `frontend-modern/src/pages/Alerts.tsx`, `frontend-modern/src/features/alerts/AlertOverviewStatsCards.tsx`, `AlertOverviewActiveAlertsSection.tsx`, `AlertOverviewAlertCard.tsx`, `useAlertAcknowledgementState.ts`, `frontend-modern/src/components/Alerts/IncidentTimelinePanel.tsx`, `IncidentEventFilters.tsx`, `InvestigateAlertButton.tsx`, and `alertAssistantHandoffModel.ts` now consume localized alert copy for the migrated journey.
- Guardrails cover catalog completeness, explicit first-wave translations, unsupported-locale fallback through the shared i18n foundation, source checks that fail if migrated Alerts Overview copy returns to hardcoded English, and machine-facing token preservation for alert identifiers, alert types, resource names, node names, source messages, commands, command output, event payloads, logs, and Assistant model-context labels.

## Verification

- `npm test -- --run src/i18n/__tests__/i18n.test.ts src/utils/__tests__/alertOverviewPresentation.test.ts src/utils/__tests__/alertTabsPresentation.test.ts src/utils/__tests__/alertActivationPresentation.test.ts src/components/Alerts/__tests__/InvestigateAlertButton.test.tsx src/features/alerts/__tests__/alertsLocalization.guardrails.test.ts src/features/alerts/__tests__/OverviewTab.emptystate.test.tsx src/features/alerts/__tests__/OverviewTab.timelineerror.test.tsx src/features/alerts/__tests__/OverviewTab.total24h.test.tsx` passed: 9 files, 96 tests.
- `npm run type-check` passed.
- `npm run dev:status` passed: managed dev server healthy at `http://127.0.0.1:5173` with frontend shell, proxy, and backend health checks green.
- `npm run lint:canonical-platforms` passed.
- Browser proof passed against the managed dev server at `http://127.0.0.1:5173`. `/alerts/overview` was not auth-blocked and rendered live active alerts. German proof rendered `Warnmeldungsuebersicht`, `Warnmeldungen aktiviert`, `Ausgeloest (24h)`, `Aktive Warnmeldungen`, `Bestaetigen`, `Zeitleiste`, and `Pulse Assistant fragen` while preserving source alert messages such as `VM 'docker' is powered off`, alert type labels such as `(Powered Off)`, resource names such as `docker`, and node/resource strings such as `minipc`. Spanish proof used Settings > General > Language to switch to `Español`, then `/alerts/overview` rendered `Resumen de alertas`, `Alertas activadas`, `Activadas (24h)`, `Alertas activas`, `Reconocer todas (12)`, `Reconocer`, `Linea de tiempo`, and `Preguntar a Pulse Assistant` while preserving the same source alert data. Expanding one active alert timeline rendered the localized `Ocultar linea de tiempo` action and `Cargando linea de tiempo...` loading state; that selected incident request did not resolve to a loaded/empty/error timeline during the browser wait, and browser console errors were empty. Loaded timeline, unavailable timeline, filter, note, event-payload, and machine-token preservation are covered by the targeted unit tests.

## Residuals

- The broader localized-product-surfaces lane remains open. This slice does not translate website/acquisition copy, install and troubleshooting docs, Settings surfaces beyond Settings General, first-session surfaces beyond the already-migrated setup journey, core monitoring tables outside Alerts Overview, alert configuration/history surfaces outside this overview triage flow, mobile handoff copy, commercial trust surfaces, public checkout/account surfaces, or native mobile surfaces.
- Locale QA remains local guardrail and browser proof for this migrated frontend-modern journey. Broader screenshot review, native speaker review, pseudo-locale coverage, extraction tooling, and future French/Brazilian Portuguese/Japanese/Simplified Chinese/Korean expansion remain separate governed work.
- Machine-facing payloads, log text, config keys, API fields, commands, source resource names, alert identifiers, event payloads, model-context labels, and user-entered identifiers remain intentionally untranslated.
