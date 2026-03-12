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

- Real v5-installed unified agent upgraded through the candidate v6 RC asset path.
- Canonical v6 identity continuity held without duplicate or orphaned registration.
- Legacy persisted token scope compatibility held during crossover.
- `updated_from` continuity was observed once on first v6 startup and then cleared.
- User-visible agent counts remained aligned with runtime enforcement after reconnect.

## Notes

- The private RC host had to serve the clean `linux-amd64` agent artifact. Earlier rehearsal attempts failed because the temporary private asset set accidentally contained a non-Linux binary.
- The clean private RC host is now detached and running from `/tmp/pulse-rc-clean.LqaK56`, not from the repo-root dirty build.
