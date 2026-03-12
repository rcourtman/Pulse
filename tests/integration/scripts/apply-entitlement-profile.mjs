#!/usr/bin/env node

import { applyRequestedEntitlementProfile } from './entitlement-bootstrap.mjs';

try {
  await applyRequestedEntitlementProfile();
} catch (error) {
  console.error('[integration] Failed to apply entitlement profile:', error?.message || error);
  process.exit(1);
}
