import { describe, expect, it } from 'vitest';
import {
  getAgentCapabilityBadgeClass,
  getAgentCapabilityLabel,
} from '@/utils/agentCapabilityPresentation';

describe('agentCapabilityPresentation', () => {
  it('uses canonical labels for unified agent capabilities', () => {
    expect(getAgentCapabilityLabel('agent')).toBe('Agent');
    expect(getAgentCapabilityLabel('docker')).toBe('Docker');
    expect(getAgentCapabilityLabel('kubernetes')).toBe('Kubernetes');
    expect(getAgentCapabilityLabel('proxmox')).toBe('Proxmox');
  });

  it('uses canonical capability badge tones', () => {
    expect(getAgentCapabilityBadgeClass('proxmox')).toContain('amber-100');
    expect(getAgentCapabilityBadgeClass('kubernetes')).toContain('emerald-100');
    expect(getAgentCapabilityBadgeClass('agent')).toContain('blue-100');
  });
});
