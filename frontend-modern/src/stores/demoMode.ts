import type { SecurityStatus } from '@/types/config';
import {
  presentationPolicyIsDemoMode as demoModeEnabled,
  sessionPresentationPolicyResolved as demoModeResolved,
  syncSessionPresentationPolicy,
} from '@/stores/sessionPresentationPolicy';

export function syncDemoModeFromSecurityStatus(
  status?: Pick<SecurityStatus, 'sessionCapabilities' | 'presentationPolicy'> | null,
): boolean {
  return syncSessionPresentationPolicy(status).demoMode;
}

export { demoModeEnabled, demoModeResolved };
