import type { SecurityStatus } from '@/types/config';
import {
  sessionCapabilities,
  sessionCapabilitiesResolved as demoModeResolved,
  syncSessionCapabilities,
} from '@/stores/sessionCapabilities';

export function syncDemoModeFromSecurityStatus(
  status?: Pick<SecurityStatus, 'sessionCapabilities'> | null,
): boolean {
  return syncSessionCapabilities(status).demoMode;
}

export function demoModeEnabled(): boolean {
  return sessionCapabilities().demoMode;
}

export { demoModeResolved };
