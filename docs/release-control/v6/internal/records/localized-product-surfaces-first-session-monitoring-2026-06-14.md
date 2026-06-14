# Localized Product Surfaces First Session Monitoring - 2026-06-14

## Slice

- Claimed governed slice: `candidate-lane:localized-product-surfaces`.
- Migrated journey: first-session setup through the first monitoring handoff, including Runtime Home loading copy, the Setup Wizard shell and progress indicator, Welcome bootstrap-token guidance, Security admin-account creation copy, Setup Completion credential/next-step/source-choice copy, and the connected first monitored system handoff.
- First-wave locales: German (`de`) and Spanish (`es`).

## Evidence

- `frontend-modern/src/i18n/messages.ts` now includes explicit German and Spanish catalog entries for the migrated first-session monitoring journey.
- `frontend-modern/src/i18n/policy.ts` records first-session monitoring non-translation tokens for commands, URLs, API labels, generated credentials, product/source identifiers, and setup-token file names.
- `frontend-modern/src/components/SetupWizard/SetupWizard.tsx`, `StepIndicator.tsx`, `steps/WelcomeStep.tsx`, `steps/SecurityStep.tsx`, `SetupCompletionPanel.tsx`, and `setupCompletionModel.ts` now render migrated operator copy through the i18n owner layer instead of page-local English.
- `frontend-modern/src/pages/RuntimeHome.tsx` now renders the authenticated workspace handoff through the same catalog.
- Guardrails cover catalog completeness, explicit first-wave translations, unsupported-locale fallback through the shared i18n foundation, machine-facing token preservation, localized render proof for German and Spanish setup states, and source checks that fail if migrated first-session copy returns to hardcoded English in the touched surfaces.

## Verification

- `npm --prefix frontend-modern test -- --run src/i18n/__tests__/i18n.test.ts src/components/SetupWizard/__tests__/SetupWizard.localization.test.tsx src/components/SetupWizard/__tests__/SetupCompletionPanel.guardrails.test.ts src/components/SetupWizard/__tests__/WelcomeStep.test.tsx src/components/SetupWizard/__tests__/SetupCompletionPanel.test.tsx src/components/SetupWizard/__tests__/SetupCompletionPreview.test.tsx src/components/SetupWizard/__tests__/SetupWizard.test.tsx src/pages/__tests__/RuntimeHome.test.tsx` passed: 8 files, 44 tests.
- `npm --prefix frontend-modern test -- --run src/components/SetupWizard/__tests__/SetupWizard.localization.test.tsx src/i18n/__tests__/i18n.test.ts` passed: 2 files, 18 tests.
- `npm --prefix frontend-modern run type-check` passed.
- Browser proof passed against the managed dev server at `http://127.0.0.1:5173`: `/settings/system-general` switched the app language through the visible `Español` and `Deutsch` controls; `/preview/setup-complete` rendered the empty first-source handoff in Spanish with `Elige tu primera fuente de infraestructura`, `Credenciales que debes guardar ahora`, `Agregar infraestructura`, and `Instalar Pulse Agent`; `/preview/setup-complete?scenario=vmware-api-backed` rendered the connected handoff in German with `Erstes ueberwachtes System verbunden` and `Infrastruktur oeffnen` while preserving `vCenter Prod`, `VMware vSphere`, `API`, and `Pulse Agent`.

## Residuals

- The broader localized-product-surfaces lane remains open. This slice does not translate website/acquisition copy, install and troubleshooting docs outside the first-session UI, Settings surfaces beyond Settings General, core monitoring tables, alerts/operator-action copy, mobile handoff copy, commercial trust surfaces, public checkout/account surfaces, or native mobile surfaces.
- Locale QA remains local guardrail and browser proof for the migrated frontend-modern journey. Broader screenshot review, native speaker review, pseudo-locale coverage, extraction tooling, and future French/Brazilian Portuguese/Japanese/Simplified Chinese/Korean expansion remain separate governed work.
- Machine-facing payloads, log text, config keys, API fields, commands, source resource names, URLs, generated credentials, and user-entered identifiers remain intentionally untranslated.
