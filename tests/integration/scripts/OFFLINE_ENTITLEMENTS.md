# Offline Organization provisioning

This fixture is only for a source-built, non-release Community backend on the
same host as Playwright. It is not an installation workaround or a replacement
for private runtime implementations. No production account, licensing service,
admin endpoint or signing material is used.

After the normal locked installs in `tests/integration`, `frontend-modern` and
`internal/cloudcp/portal/frontend`, and Chromium installation, run from
`tests/integration`:

```sh
PULSE_E2E_USE_LOCAL_BACKEND=true node scripts/with-offline-entitlements.mjs \
  node scripts/run-playwright.mjs \
  --config=playwright.multi-tenant-diagnostic.config.ts \
  --grep 'Scenario 1:|create, update, member manage' --workers=1 --retries=0
```

The wrapper generates an ephemeral Ed25519 issuer, binds it to loopback and
keeps it alive for the complete managed backend/browser lifecycle. The backend
is freshly built at a unique temporary path. Its development trust root and
licensing URL point to the issuer; signature bypass and mock mode are false.
The issuer has no outbound requests, redirect responses or production fallback.
It implements only activation and authenticated status/refresh, rejects legacy
exchange, and binds independent installation credentials to fingerprints.
Repeated activation for one fingerprint is idempotent. Issuer state lives for
this run, including backend restarts; restarting the issuer requires a fresh
run, not reusing an old activation directory. Never export fixture keys, grant
JWTs, installation credentials or backend runtime-state files as artifacts.

The Organization spec activates default through the authenticated API before
each scenario. `createOrg` independently activates each created organisation
with explicit scope headers. Capability assertions require `multi_tenant`,
exclude Community's private `rbac`, and check the security contract's
`sessionCapabilities.demoMode=false`. Offline activation suppresses the old
billing.json profile writer; there is no direct capability injection.
No offline key means the existing unentitled and other test paths are unchanged.

The separate CI provisioning job runs two narrow scenarios and issuer boundary
tests. It does not change stable/probation/quarantine membership. Passing it is
not Organization sharing acceptance, private-runtime RBAC coverage, full-suite
acceptance or release qualification. To investigate another Organization
scenario, use its explicit grep in the same diagnostic config and retain its
result separately. The Web owner judges those user-interface outcomes.

Local proof on 9 September 2026: two Chromium scenarios passed (3.8s test time)
on the final wrapper, with real authenticated activation and created-org scope;
13 focused Node cases passed for signatures, three distinct identities,
cross-identity/fingerprint/credential rejection, unsupported routes, ephemeral
keys, wrapper lifecycle and refusal of remote/release inputs. No hosted run or
release delivery is established by this proof.
