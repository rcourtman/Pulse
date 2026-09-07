# Unsaved edit retention — 7 September 2026

## Settings-parent qualification

`scripts/check-recovery-feedback.mjs` now mounts the real
`AlertsConfigurationSurface` for Destinations. Its
`useAlertsConfigurationState`, configuration snapshot, overrides and
destination hooks are unmocked. Only the API boundary and external resource /
activation inputs are scripted; the enclosing fixture supplies the dirty
signal as the application shell would.

The test waits for initial configuration loading, enters a synthetic
`example.invalid` ping URL, and checks the rendered value, dirty signal and
real unsaved-changes banner survive:

- rejected retry and toast expiry;
- cancelled dismissal;
- accepted retry followed by unavailable health refresh;
- subsequent recovery, failed dismissal, healthy-card removal and message clearing.

At each checkpoint, configuration was read only once and neither the global
configuration nor ping URL was saved. Finally, clicking the real Save Changes
button sends the exact edited URL through the real destination save path to
the scripted API, writes global configuration once and clears dirty state.
This verifies parent-owned retention rather than just an isolated input.

Validation: two serialized Chromium runs passed all 12 cases (Overview and
Destinations at 1440, 900 and 390 pixels in light/dark), including six
settings-parent edit/save cases. The final formatted-script result is
`settings-parent-result.json`; `settings-parent-source.sha256` identifies the
script. `node --check` and `git diff --check` passed.

Earlier attempts are not passes: the first could not start because this
isolated workspace lacked Vite dependencies; lockfile installs resolved that.
The next timed out because the test looked for an input of type URL, whereas
the existing field is masked. Selecting its existing ID prefix resolved the
fixture error. No runtime code was changed.

## Boundaries

The previous fixture used a synthetic reactive parent directly around
DestinationsTab. This replaces that limitation with the real settings surface
and state, but does not mount the entire Alerts application shell or exercise
navigation, organisation switching, concurrent configuration loads, backend
persistence, installed recovery, history retention, recipient receipt or
assistive-technology announcements. No release qualification is claimed.
Historical screenshots and older receipts are preserved, not refreshed or
presented as visual acceptance for this test-only change.

Independent rationale: W3C's
[redundant-entry guidance](https://www.w3.org/WAI/WCAG22/Understanding/redundant-entry.html),
successfully retrieved directly on 7 September, explains the effort and error
risk of requiring people to recall and re-enter information. This supports
input-retention testing, not new product demand or a WCAG conformance claim.
