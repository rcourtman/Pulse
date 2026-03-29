# Pulse Account Frontend Dev Loop

The production portal is embedded into the control-plane binary, so styling changes do not appear on `cloud.pulserelay.pro` until the frontend bundle is rebuilt and the control-plane is redeployed.

For local UI work, use the preview server instead of production.

## Commands

```bash
npm --prefix internal/cloudcp/portal/frontend run dev
```

This does three things:

- builds the current portal frontend with the same esbuild pipeline used for production
- watches `internal/cloudcp/portal/frontend/src`
- serves a local preview at `http://127.0.0.1:8765`

## Preview URLs

- Managed hosted account: `http://127.0.0.1:8765/?scenario=managed`
- Read-only hosted account: `http://127.0.0.1:8765/?scenario=readonly`
- Self-hosted commercial account: `http://127.0.0.1:8765/?scenario=selfhosted`
- Empty hosted account: `http://127.0.0.1:8765/?scenario=empty`

Append `&reset=1` to reset a scenario back to its seed state.

The preview server keeps scenario state in memory and stubs the main portal APIs so you can exercise:

- workspace create/suspend/delete
- access invite/change-role/remove
- hosted billing handoff
- self-hosted billing/license/privacy flows

The preview server is intentionally local-only. It is for design and interaction iteration, not for validating production credentials or external services.

## Production Build

When the UI is ready to ship, rebuild the embedded bundle:

```bash
npm --prefix internal/cloudcp/portal/frontend run build
```

That updates:

- `internal/cloudcp/portal/dist/portal_app.js`
- `internal/cloudcp/portal/dist/portal_app.css`
- `internal/cloudcp/portal/dist/build_manifest.json`

After that, rebuild and deploy the control-plane binary/image to see the change on the live portal.

## Functional Caveat

The preview harness is good for local front-end iteration, but it is still a harness. Final verification for auth, live account state, Stripe, and production data should happen against a real control-plane instance before shipping.
