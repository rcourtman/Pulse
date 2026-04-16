# Self-Hosted Core Monitoring Free Direction

Date: 2026-04-16
Target: `v6-rc-stabilization`
Lanes: `L2`, `L3`, `L9`

## Decision

Pulse v6 self-hosted core monitoring is not a paid gate.

- Homelab users must not be monetized on monitored-system volume itself.
- `monitored systems` remains the canonical counted unit for product understanding, migration truth, and any optional commercial UX that still needs usage context.
- Paid self-hosted value should come from optional extras, hosted convenience, business workflow, support, or similar non-core surfaces rather than from gating core monitoring coverage.

## Why

- The capped RC1 self-hosted ladder puts commercial pressure on the same serious self-hosted users who are the natural Pulse adoption base.
- The product goal is for self-hosted Pulse to feel useful and generous for normal and even fairly serious homelab use.
- Open-source core monitoring should stay free, while paid value comes from extras that are meaningfully optional or business-oriented.

## Consequences

- The capped 2026-03-17 self-hosted model lock is superseded as the final GA direction and remains only as historical RC1 context.
- Current RC-era self-hosted cap ladders, copy, and pricing records must not be treated as settled GA truth just because they already shipped in `v6.0.0-rc.1`.
- Before GA, Pulse still needs one governed packaging decision for which self-hosted extras remain paid and how Relay, Pro, Pro+, or successor plans map to that non-cap model.
- Existing lifetime and grandfathered recurring continuity remain unaffected: they stay valid and uncapped.
