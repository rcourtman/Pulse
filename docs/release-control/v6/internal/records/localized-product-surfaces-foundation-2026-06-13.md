# Localized Product Surfaces Foundation - 2026-06-13

## Slice

- Claimed governed slice: `candidate-lane:localized-product-surfaces`.
- Migrated journey: Settings -> System -> General language preference, including the Settings shell/header/navigation copy already on the catalog path, Appearance language selection, pseudonymous telemetry controls, Docker / Podman update-action controls, and monitoring cadence copy.
- First-wave locales: German (`de`) and Spanish (`es`).

## Evidence

- `frontend-modern/src/i18n/locales.ts` owns locale normalization, supported locale registry metadata, first-wave locale declaration, and English fallback-chain behavior.
- `frontend-modern/src/i18n/messages.ts` owns the English source catalog plus complete German and Spanish first-wave entries for the migrated Settings General journey.
- `frontend-modern/src/i18n/policy.ts` records catalog shape and non-translation rules for API fields, config keys, commands, logs/payloads, resource names, user-entered identifiers, and product/integration identifiers.
- `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx` and `DockerRuntimeSettingsCard.tsx` now render migrated Settings General copy through the i18n owner layer instead of page-local English literals.
- Guardrails cover catalog completeness, unsupported-locale fallback, explicit first-wave translations for the migrated journey, stable machine-facing tokens (`API`, `CPU`, `IP`, `JSON`, env vars, `Pulse`, `Proxmox VE`, `Docker / Podman`, and `"Update"`), and source checks that fail if the migrated Settings General copy returns to hardcoded English.

## Verification

- `npm --prefix repos/pulse/frontend-modern test -- --run src/i18n/__tests__/i18n.test.ts src/components/Settings/__tests__/GeneralSettingsPanel.localization.test.tsx src/components/Settings/__tests__/GeneralSettingsPanel.guardrails.test.ts src/utils/__tests__/systemSettingsPresentation.test.ts src/components/Settings/__tests__/settingsLocalization.test.ts` passed: 5 files, 35 tests.
- `npm --prefix repos/pulse/frontend-modern test -- --run src/components/Settings/__tests__/settingsArchitecture.test.ts src/i18n/__tests__/i18n.test.ts` passed: 2 files, 50 tests.
- `npm --prefix repos/pulse/frontend-modern run type-check` passed.
- `python3 repos/pulse/scripts/release_control/status_audit.py --pretty` passed with no errors.
- `python3 repos/pulse/scripts/release_control/control_plane_audit.py --check` passed with no errors or warnings.
- Browser proof used the managed dev server at `http://127.0.0.1:5173/settings/system-general`. English Settings General loaded without an auth blocker. Switching to Spanish rendered `Ajustes`, `Apariencia`, `Datos de uso y privacidad`, `Telemetría saliente anónima`, `Vista previa del payload`, `Actualizaciones de Docker / Podman`, and `Cadencia de supervisión`, while preserving `Docker / Podman`, `"Update"`, `API`, `CPU`, and `PULSE_DISABLE_DOCKER_UPDATE_ACTIONS=true`. Switching to German rendered `Einstellungen`, `Darstellung`, `Nutzungsdaten und Datenschutz`, `Anonyme ausgehende Telemetrie`, `Payload anzeigen`, `Docker / Podman-Updates`, and `Monitoring-Takt`, while preserving the same machine-facing tokens.

## Residuals

- The broader localized-product-surfaces lane remains open. This slice does not translate website/acquisition copy, install and troubleshooting docs, setup wizard/first-run pages outside Settings General, core monitoring tables and alerts/operator-action copy, mobile handoff copy, commercial trust surfaces, or public checkout/account surfaces.
- Locale QA remains local guardrail coverage only for the migrated frontend-modern journey. Broader screenshot review, native speaker review, pseudo-locale coverage, extraction tooling, and future French/Brazilian Portuguese/Japanese/Simplified Chinese/Korean expansion remain separate governed work.
- Machine-facing payloads, log text, config keys, API fields, commands, source resource names, and user-entered identifiers remain intentionally untranslated.
