# Pulse v6.0.0 Release Notes

Pulse v6 is a major architectural update focused on unified resources, a reorganized product surface, newer licensing and entitlement flows, and the foundations for relay, mobile, storage, and hosted-mode improvements. For RC1, the aim is simple: let a small set of interested users try it early and report what still needs work before GA.

## RC1 Preview Notes

`v6.0.0-rc.1` is a low-key release candidate intended for a small group of keen users who want to try Pulse v6 early and send feedback before GA.

- I do not recommend upgrading a production Pulse v5 installation yet.
- If you want to test v6, use a staging instance, lab environment, or separate non-production install first.
- Existing Pulse Pro users with valid v5 Pro/Lifetime licenses can test the RC: upgraded installs can auto-exchange persisted v5 licenses into the v6 activation model, and the v6 license panel can also accept a valid v5 key as migration input if needed.
- If you try RC1, the most useful outcome is feedback about onboarding, upgrade friction, regressions, and anything that feels unfinished or unreliable.
- This RC note is intentionally brief. I will publish the fuller v6 changelog with the final GA release.

---

## Pulse v5 Support Transition

The first stable `v6.0.0` release will publish these exact calendar dates once
the RC-to-GA gate is actually cleared:

- Pulse v5 entered maintenance-only support on `2026-03-24`.
- I will ship only critical security, critical correctness or data-loss,
  installer or updater failure, licensing or billing blocker, and safe
  migration blocker fixes for existing v5 users until `2026-06-22`.
- After `2026-06-22`, Pulse v5 is end-of-support and new fixes land on v6
  unless I publish an explicit exception.

This notice must stay aligned with
`docs/release-control/v6/V5_MAINTENANCE_SUPPORT_POLICY.md`.

---

## What To Try

If you install RC1, the most useful feedback is around these flows:

- upgrading a non-production v5 install to v6
- first-session onboarding and general navigation
- unified Infrastructure, Workloads, Storage, and Recovery views
- Pulse Pro activation or v5 license migration
- Relay/mobile pairing if you already use those features
- obvious regressions, missing data, rough edges, or anything that feels unreliable

## Highlights

- **Unified v6 resource model and navigation**: Pulse now centers the product around canonical unified resources and task-oriented views instead of the older platform-specific top-level surfaces.
- **New dashboard and page structure**: Dashboard, Infrastructure, Workloads, Storage, and Recovery are the main v6 surfaces, with updated search, keyboard navigation, and mobile navigation behavior.
- **Entitlements and licensing rebuilt for v6**: Pulse now uses a fuller entitlement model, including v5 Pro/Lifetime license exchange into the v6 activation flow.
- **Relay and mobile groundwork**: Remote access, pairing, and push-notification plumbing are now part of the v6 product surface.
- **TrueNAS, storage, and recovery improvements**: v6 expands storage visibility and recovery tracking while bringing more infrastructure into the same model.
- **Hosted and multi-tenant foundations**: The core hosted-mode and organization boundaries are in place, but these remain opt-in and are not the main point of this RC.

## Notable Changes For Existing Users

- **Navigation changed substantially**: v6 is organized around Dashboard, Infrastructure, Workloads, Storage, and Recovery.
- **Pulse Pro activation changed**: existing v5 Pro/Lifetime licenses can migrate into the v6 activation flow.
- **Remote access and mobile groundwork are now part of the product surface**: if you use relay features already, RC1 feedback there is especially useful.
- **This is still a preview build**: expect rough edges, incomplete polish, and some regressions compared with a mature v5 install.

---

## More Detail

If you want the operator-facing upgrade and migration details, use these docs instead of this RC summary:

- `docs/UPGRADE_v6.md`
- `docs/PULSE_PRO.md`
- `docs/MIGRATION_UNIFIED_NAV.md`
