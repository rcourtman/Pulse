# Runner isolation diagnostic — 5 September 2026

Base: 319b52a1efb291146bffdc28960a5ed32579ee10 plus accompanying harness repair.
No application code changed.

Fresh external reference: https://playwright.dev/docs/auth recommends isolated
state for tests modifying server state. This is qualification maintenance,
not new product demand.

Validation:
- 16 focused Node runner image/cleanup checks passed, including inherited
  report/results/browser URL override regression checks.
- Two simultaneous shell multi-tenant invocations within one pulse-heavy-run
  command built and started real Docker stacks, then failed because this
  checkout lacked @playwright/test. Both cleaned up.
- Installed locked integration dependencies with npm ci --ignore-scripts.
  Repeated simultaneous invocations under pulse-heavy-run. Both reached healthy
  stacks and entitlement bootstrap, then returned 1: No tests found.
  e2e-tiering.mjs explicitly quarantines 03-multi-tenant.spec.ts for chromium.
  Quarantine was not removed or bypassed.
- Scoped docker inspect captured the identities below without credentials.
  Cleanup logs report removal of each server/mock/seed, network and data volume.

/pulse-e2e-a460ce699143f7d50f46eed8aeaf8f03-mock image=sha256:0d673a0b6ab8352142d39f59ac7c9614f38c98b52b33b009bcdb33ba055e5307 project=pulse-e2e-a460ce699143f7d50f46eed8aeaf8f03 ports={"8080/tcp":[{"HostIp":"127.0.0.1","HostPort":"32780"}]} status=running
/pulse-e2e-a460ce699143f7d50f46eed8aeaf8f03-server image=sha256:7927dba54e1ccb5f6c549ef5ce996bfca9c5e414381859678589ffeb0250ac0d project=pulse-e2e-a460ce699143f7d50f46eed8aeaf8f03 ports={"7655/tcp":[{"HostIp":"127.0.0.1","HostPort":"32782"}],"7656/tcp":[{"HostIp":"127.0.0.1","HostPort":"32783"}]} status=running
/pulse-e2e-da316da1633b0a1fa503a680e6a75d89-mock image=sha256:51f71b5e63e4425a7e28f5e5e422d3bb134a04969d0c44e8c03792b44ff1b48c project=pulse-e2e-da316da1633b0a1fa503a680e6a75d89 ports={"8080/tcp":[{"HostIp":"127.0.0.1","HostPort":"32781"}]} status=running
/pulse-e2e-da316da1633b0a1fa503a680e6a75d89-server image=sha256:1b5561f58972f3c5dd11a5428c72d6d72fa933d6fec703a1cf9ff8a6280949cb project=pulse-e2e-da316da1633b0a1fa503a680e6a75d89 ports={"7655/tcp":[{"HostIp":"127.0.0.1","HostPort":"32784"}],"7656/tcp":[{"HostIp":"127.0.0.1","HostPort":"32785"}]} status=running

Limits and next work:
- This proves concurrent startup and failure-path cleanup, not browser success,
  auth-fixture execution, signal interruption, organisation admission/reconnect,
  installed-customer behaviour or release qualification.
- Direct npm startup remains shared: compose-command.mjs supplies no project,
  Compose uses fixed default container names, and entitlement/helpers default to
  pulse-test-server. run-playwright.mjs scopes runtime state but not report roots.
  Do not advertise direct npm invocations as concurrency-safe.
- Other specs still store authentication in tests/tmp/playwright-auth.
  This repair is limited to the shell runner and multi-tenant fixture.
- Resolve the multi-tenant qualification entry point explicitly without silently
  weakening stable-tier quarantine, then complete the browser matrix on exact
  image identities. Retain identities/ports automatically rather than polling.
