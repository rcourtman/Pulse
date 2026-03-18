import { describe, expect, it } from 'vitest';
import {
  getClusterEndpointPresentation,
  getClusterEndpointPulseStatus,
} from '@/utils/clusterEndpointPresentation';

describe('clusterEndpointPresentation', () => {
  it('maps reachable online endpoints to success styling', () => {
    const presentation = getClusterEndpointPresentation({ online: true, pulseReachable: true });
    expect(presentation.panelClass).toContain('border-green-200');
    expect(presentation.proxmoxLabel).toBe('Online');
    expect(presentation.pulseLabel).toBe('Reachable');
  });

  it('maps unreachable pulse endpoints to warning styling', () => {
    const presentation = getClusterEndpointPresentation({ online: true, pulseReachable: false });
    expect(presentation.panelClass).toContain('border-amber-200');
    expect(presentation.pulseLabel).toBe('Unreachable');
  });

  it('maps online checking endpoints to informational styling', () => {
    const presentation = getClusterEndpointPresentation({ online: true, pulseReachable: null });
    expect(presentation.panelClass).toContain('border-blue-200');
    expect(presentation.pulseLabel).toBe('Checking...');
  });

  it('maps offline endpoints to muted styling', () => {
    const presentation = getClusterEndpointPresentation({ online: false, pulseReachable: null });
    expect(presentation.panelClass).toContain('bg-surface-alt');
    expect(presentation.proxmoxLabel).toBe('Offline');
  });

  it('normalizes pulse reachability to canonical statuses', () => {
    expect(getClusterEndpointPulseStatus({ pulseReachable: true })).toBe('reachable');
    expect(getClusterEndpointPulseStatus({ pulseReachable: false })).toBe('unreachable');
    expect(getClusterEndpointPulseStatus({ pulseReachable: undefined })).toBe('checking');
  });
});
