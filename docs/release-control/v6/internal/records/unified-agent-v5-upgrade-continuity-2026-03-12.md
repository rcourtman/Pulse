# Unified Agent v5 Upgrade Continuity Record

- Date: `2026-03-12`
- Gate: `unified-agent-v5-upgrade-continuity`
- Assertion: `RA9`
- Environment:
  - Private RC host: `http://192.168.0.98:7655`
  - Host under test: `delly`
  - Candidate version: `v6.0.0-rc.1`
  - Starting version: `5.1.23`

## Automated Proof Baseline

- `python3 scripts/release_control/unified_agent_rc_rehearsal.py --base-url http://127.0.0.1:7655 --expected-version 6.0.0-rc.1 --release-base-url file:///tmp/pulse-private-release-assets --arch linux-amd64 --api-token <redacted> --expected-active-agents 3 --expected-agent-name delly --expected-online-agents 3`
- Result: pass

## Automated Local Runtime Proof

- Date: `2026-06-03`
- Host architecture: `darwin-arm64`
- Starting binary: `v5.1.34` built from `repos/pulse-5.1.x` tag `v5.1.34`
- Target binary: `v6.0.0-rc.6` built from `pulse/v6-release`
- Command: `PYTHONPATH=scripts/release_control/internal python3 scripts/release_control/internal/unified_agent_rc_rehearsal.py --base-url http://127.0.0.1:1 --expected-version 6.0.0-rc.6 --release-base-url http://127.0.0.1:1/releases --skip-asset-checks --runtime-v5-agent /tmp/pulse-v5-v6-runtime-proof/bin/pulse-agent-v5.1.34-darwin-arm64 --runtime-v6-agent /tmp/pulse-v5-v6-runtime-proof/bin/pulse-agent-v6.0.0-rc.6-darwin-arm64 --runtime-expected-from v5.1.34 --runtime-expected-to v6.0.0-rc.6 --runtime-timeout 90 --json`
- Result: pass
- Observed result: the v5 process reported `v5.1.34`, downloaded v6 binary checksum `407d803270388e36ddc6c851c0dd04bdb2ff9e4d7c08cda9d4cf5839df790227`, exec'd into `v6.0.0-rc.6`, and reported `updated_from=v5.1.34`.
- Scope note: this local proof covers the in-place process swap, checksum-gated download, v5 host-report compatibility, v6 agent-report compatibility, and first-report `updated_from`. Live active-agent accounting still needs the authenticated Pulse API rehearsal above when proving a full server environment.

## Manual Crossover Exercise

1. Built a real `linux-amd64` v5 agent from `main` with version `5.1.23`.
2. Stopped the normal `pulse-agent.service` on `delly`.
3. Launched the v5.1.23 agent manually against the private RC host at `http://192.168.0.98:7655`.
4. Observed the real updater detect `availableVersion=6.0.0-rc.1`.
5. Observed the process restart into v6 and log:
   - `previousVersion=5.1.23`
   - `currentVersion=v6.0.0-rc.1`
6. Confirmed `/usr/local/bin/.pulse-update-info` was consumed and cleared after first v6 startup.
7. Confirmed the server moved from legacy `POST /api/agents/host/report` traffic to canonical `POST /api/agents/agent/report` for `delly`.
8. Confirmed the agent ledger recovered to one canonical `delly` identity with no duplicate registration and total active agents remained aligned at `3`.
9. Restored `delly` to the managed `pulse-agent.service` path after the rehearsal.

## Outcome

- Real v5-installed unified agent upgraded through the candidate v6 prerelease asset path.
- Canonical v6 identity continuity held without duplicate or orphaned registration.
- Legacy persisted token scope compatibility held during crossover.
- `updated_from` continuity was observed once on first v6 startup and then cleared.
- User-visible agent counts remained aligned with runtime enforcement after reconnect.

## Notes

- The private RC host had to serve the clean `linux-amd64` agent artifact. Earlier rehearsal attempts failed because the temporary private asset set accidentally contained a non-Linux binary.
- The clean private RC host is now detached and running from `/tmp/pulse-rc-clean.LqaK56`, not from the repo-root dirty build.
