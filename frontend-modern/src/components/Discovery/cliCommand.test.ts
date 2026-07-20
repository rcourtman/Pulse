import { describe, it, expect } from 'vitest';
import { deriveCliCommand } from './cliCommand';

describe('deriveCliCommand', () => {
  it('derives pct exec for a native LXC', () => {
    expect(
      deriveCliCommand(
        'system-container',
        '101',
        'Use pulse_control ... directly inside the container.',
      ),
    ).toBe('pct exec 101 -- bash');
  });

  it('layers docker exec for a service in a nested container (HA-in-LXC)', () => {
    const cliAccess =
      'Use pulse_control ... The service runs inside a Docker container named "homeassistant" ' +
      '— prefix commands with: docker exec homeassistant <your-command>.';
    expect(deriveCliCommand('system-container', '101', cliAccess)).toBe(
      'pct exec 101 -- docker exec homeassistant bash',
    );
  });

  it('uses docker exec for a Docker container', () => {
    expect(deriveCliCommand('app-container', 'redis', undefined)).toBe('docker exec redis bash');
  });

  it('returns null for types without a clean human command (VM, k8s, agent)', () => {
    expect(deriveCliCommand('vm', '200', 'Use pulse_control ... inside the VM.')).toBeNull();
    expect(
      deriveCliCommand('pod', 'web-0', 'Use kubectl exec -n default web-0 -- <cmd>'),
    ).toBeNull();
    expect(deriveCliCommand('agent', 'node1', 'Use pulse_control ...')).toBeNull();
  });

  it('returns null when the resource id is missing', () => {
    expect(deriveCliCommand('system-container', '', 'x')).toBeNull();
  });
});
