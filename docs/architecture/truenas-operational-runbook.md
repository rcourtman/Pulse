# TrueNAS Operational Runbook

This runbook defines operator procedures for TrueNAS enablement, disablement, immediate deactivation (kill-switch), code rollback, and data cleanup.

## Prerequisites

1. Ensure Pulse is reachable at `http://localhost:7655`.
2. Ensure `curl`, `jq`, `git`, and `go` are installed.
3. Ensure you can restart the Pulse process in your environment (systemd, container, or dev runner).
4. Ensure you have admin API access for write operations:
   - `POST /api/truenas/connections`
   - `POST /api/truenas/connections/test`
   - `DELETE /api/truenas/connections/:id`
5. Set an optional auth header for protected endpoints if your environment requires it:

```bash
AUTH_HEADER=(-H "Authorization: Bearer $PULSE_TOKEN")
```

## 1) Enable Path

1. Set the feature flag:

```bash
export PULSE_ENABLE_TRUENAS=true
# Also accepted (case-insensitive): 1, yes, on
```

2. Restart Pulse.

3. Verify the TrueNAS connections endpoint is enabled and returns an array:

```bash
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/truenas/connections | jq .
# Expected: [] (no configured connections yet) OR existing configured connections
```

4. Verify logs for poller startup activity:

```bash
journalctl -u pulse -n 200 --no-pager | rg -i "TrueNAS poller started|TrueNASPoller"
```

5. Add a connection:

```bash
curl -s "${AUTH_HEADER[@]}" \
  -X POST http://localhost:7655/api/truenas/connections \
  -H "Content-Type: application/json" \
  -d '{"name":"nas-1","host":"truenas.local","api_key":"REPLACE_ME"}' | jq .
```

6. Test the connection:

```bash
# Current implementation endpoint:
curl -s "${AUTH_HEADER[@]}" \
  -X POST http://localhost:7655/api/truenas/connections/test \
  -H "Content-Type: application/json" \
  -d '{"name":"nas-1","host":"truenas.local","api_key":"REPLACE_ME"}' | jq .

# Contract path (if available in your branch):
# POST /api/truenas/connections/:id/test
```

7. Verify TrueNAS resources appear:

```bash
# Unified v2 (preferred for source-level verification)
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/v2/resources \
  | jq '.data[] | select((.sources // []) | index("truenas"))'

# Legacy view
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/resources \
  | jq '.resources[] | select(.platformType == "truenas")'
```

## 2) Disable Path

1. Disable the feature flag:

```bash
unset PULSE_ENABLE_TRUENAS
# Or explicitly disable:
export PULSE_ENABLE_TRUENAS=false
```

2. Restart Pulse.

3. Verify the poller is NOT running by checking logs — no poll activity should appear:

```bash
# The poller gate at truenas_poller.go:48 prevents Start() when flag is off.
# Note: The connections API still responds (handlers are always registered),
# but the poller does not poll, so no new resource data is ingested.
journalctl -u pulse -n 200 --no-pager | rg -i "TrueNAS poll"
# Expected: no recent poll activity
```

4. Wait at least 120 seconds for TrueNAS source staleness transition.

5. Verify source status transitions to stale and aggregate status becomes warning:

```bash
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/v2/resources | jq '
  .data[]
  | select((.sources // []) | index("truenas"))
  | {
      id,
      name,
      resourceStatus: .status,
      truenasSourceStatus: (.sourceStatus.truenas.status // "missing")
    }'
# Expected after threshold: truenasSourceStatus == "stale", resourceStatus typically "warning"
```

## 3) Kill-Switch (Immediate Deactivation Without Restart)

1. List current TrueNAS connection IDs:

```bash
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/truenas/connections | jq -r '.[].id'
```

2. ⚠️ Delete all TrueNAS connections:

```bash
for id in $(curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/truenas/connections | jq -r '.[].id'); do
  curl -s "${AUTH_HEADER[@]}" -X DELETE "http://localhost:7655/api/truenas/connections/$id" | jq .
done
```

3. Wait for `syncConnections()` to run on the next tick (poll interval: 60 seconds).

4. Verify no configured connections remain:

```bash
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/truenas/connections | jq .
# Expected: []
```

5. Wait up to 120 seconds more and verify TrueNAS source status becomes stale:

```bash
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/v2/resources | jq '
  .data[]
  | select((.sources // []) | index("truenas"))
  | {id, name, truenasSourceStatus: (.sourceStatus.truenas.status // "missing")}'
```

6. Confirm no restart or env var change was required for this deactivation path.

## 4) Code Rollback

1. Identify rollback boundaries:
   - TrueNAS implementation range: `f9680ef8...687ecd79` (TN-00 through TRR-00)
   - First feature commit: `100494a7` (TN-01, REST client scaffold)

2. ⚠️ Revert the TrueNAS range:

```bash
git revert --no-edit f9680ef8^..687ecd79
```

3. Resolve conflicts if any, then verify the repository builds cleanly:

```bash
go build ./...
# Required: exit code 0
```

4. Restart Pulse and verify TrueNAS routes are absent/disabled.

5. Verify frontend behavior:
   - TrueNAS-specific badges and filters should no longer render (UI is additive and should self-prune once backend support is removed).

## 5) Data Cleanup

1. Determine the active data directory:

```bash
echo "${PULSE_DATA_DIR:-/etc/pulse}"
```

2. Locate the TrueNAS config file:
   - Default: `/etc/pulse/truenas.enc`
   - Dev: `tmp/dev-config/truenas.enc`

3. Confirm file exists before cleanup:

```bash
ls -l "${PULSE_DATA_DIR:-/etc/pulse}/truenas.enc"
```

4. ⚠️ Remove only `truenas.enc`:

```bash
rm -f "${PULSE_DATA_DIR:-/etc/pulse}/truenas.enc"
```

5. Verify cleanup result:

```bash
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/truenas/connections | jq .
# Expected: [] (LoadTrueNASConfig() returns empty slice when file is missing)
```

6. Verify encryption key remains intact:

```bash
ls -l "${PULSE_DATA_DIR:-/etc/pulse}/.encryption.key"
```

7. Do not delete `.encryption.key`; remove only `truenas.enc`.

## Quick Reference

| Operation | Primary Action | Restart Required | Propagation Window | Verification |
|---|---|---|---|---|
| Enable | `PULSE_ENABLE_TRUENAS=true` | Yes | Immediate after restart | `GET /api/truenas/connections` returns `[]` or entries |
| Disable | unset or `PULSE_ENABLE_TRUENAS=false` | Yes | 120s staleness window | Poller stops (no poll activity in logs); v2 resources show `sourceStatus.truenas.status="stale"` after threshold |
| Kill-Switch | Delete all `/api/truenas/connections/:id` | No | 60s provider removal + 120s staleness | Connections list becomes `[]`; v2 resources show stale TrueNAS source |
| Code Rollback | `git revert --no-edit f9680ef8^..687ecd79` | Yes (apply change) | Immediate after deploy | `go build ./...` exits `0`; TrueNAS UI/backend paths removed |
| Data Cleanup | Remove `truenas.enc` only | No (API reads empty config) | Immediate | `GET /api/truenas/connections` returns `[]`; `.encryption.key` still exists |

