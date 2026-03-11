# Cloud Paid Contract

## Purpose

Own cloud plan/version semantics, entitlement limits, hosted billing/runtime
agreement, and cloud-specific enforcement rules.

## Canonical Files

1. `pkg/licensing/features.go`
2. `pkg/licensing/evaluator.go`
3. `pkg/licensing/service.go`
4. `pkg/licensing/stripe_subscription.go`
5. `frontend-modern/src/pages/CloudPricing.tsx`

## Extension Points

1. Add or change limits through `pkg/licensing/`
2. Add or change cloud plan presentation through `CloudPricing.tsx`
3. Add contract tests where runtime and pricing need to stay aligned

## Forbidden Paths

1. New ad hoc plan names in runtime or UI
2. Silent aliases between old and new limit keys in live runtime paths
3. Pricing/UI claims that are not enforced by runtime entitlements

## Completion Obligations

1. Update this contract when cloud plan semantics change
2. Update runtime and frontend tests together when plan/limit rules move
3. Add or tighten drift tests when a pricing/runtime mismatch is fixed

## Current State

Cloud paid readiness is materially behind architecture work. The main concern is
contract coherence between pricing, entitlements, and runtime enforcement.
Legacy Cloud plan aliases are now expected to canonicalize to the `cloud_*`
contract not only when Stripe metadata is parsed, but also when persisted plan
versions are consumed at hosted entitlement and workspace-limit enforcement
boundaries.
