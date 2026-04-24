# Self-Hosted Commercial GA Coherence: Paid Runtime Docker Policy

Date: 2026-04-24
Owner: self-hosted-commercial-ga-coherence
Evidence tier: managed-runtime-exercise

## Decision

The paid self-hosted v6 GA runtime path is the private Pulse Pro archive
delivered through the license-gated download broker. Docker/image-based paid
self-hosted upgrades are explicitly deferred until a separate paid Pro image
exists, is smoke-tested, and has its own customer-safe instructions.

The public community container image remains a community delivery path and must
not be described as including the paid Pro runtime.

## Evidence

- `repos/pulse-pro/docs/migration/paid-v6-upgrade-runbook.md` records the
  archive-first paid runtime decision and the Docker/image deferral rule.
- `repos/pulse-pro/OPERATIONS.md` records the same operational policy beside
  the private R2 broker setup.
- `repos/pulse-pro/V6_LAUNCH_CHECKLIST.md` now treats the Docker/container
  strategy as decided for GA: private Pro archive first, paid Docker image
  deferred until separately proved.
- `repos/pulse-pro/scripts/validate_paid_runtime_distribution.py` validates
  that the support playbook, customer FAQ, release-day migration email, private
  download page, and v6 license-delivery email keep paid users on the private
  Pro archive path and do not present public community Docker/images or public
  community archives as paid Pro delivery.

## Proof Commands

From `/Volumes/Development/pulse/repos/pulse-pro`:

```bash
python3 scripts/validate_paid_runtime_distribution.py
```

Result:

```text
paid runtime distribution validation passed
  paid path: private Pulse Pro archive via license-gated download broker
  docker: deferred for paid self-hosted v6 until a separate paid image is proved
```

From `/Volumes/Development/pulse/repos/pulse-pro/license-server`:

```bash
GOTOOLCHAIN=go1.25.9+auto go test . -run 'TestV6LicenseEmailIncludesPrivateDownloadPage|TestPulseProDownload' -count=1
```

Result:

```text
ok  	github.com/rcourtman/pulse-pro/license-server	0.492s
```

## Result

This closes the paid-runtime Docker/container strategy gap for v6 GA without
creating a release, tag, GitHub Release, public download, Docker image, or v6
cutover. Remaining release-day work is still the actual GA artifact publication
and public track flip when release approval is granted.
