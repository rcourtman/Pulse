# Docker restart intelligence lab

This ignored-artifact harness exercises one uniquely labelled disposable Docker
container, volume, and network in the existing Colima VM. It refuses any
implicit or non-Colima Docker context, uses the existing `alpine:3.20` fixture,
and removes only resources with the exact run label.

Run only after explicit local-lab authorization:

```sh
python3 scripts/intelligence_lab/docker_restart_colima.py --run-id task06-20260712-090000
```

Evidence is written beneath `tmp/intelligence-lab/<run-id>/`, which is ignored.
Only the explicit redacted JSON, screenshot/trace, report, and checksum
basenames are retained there. Transient Pulse and Unified Agent databases live
under `tmp/intelligence-lab-scratch/<run-id>/` and are deleted before the
artifact allowlist and checksum manifest are accepted.
The second cleanup must be a no-op and the preexisting resource inventory must
be unchanged. Container restart has no product rollback; fixture deletion is
lab compensation only.
