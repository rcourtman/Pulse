/**
 * Branch-coverage tests for the exported label/path helpers in
 * availabilitySettingsModel. Each block targets a single function and drives
 * both arms of every conditional / ternary / switch / optional-chain / guard
 * branch reachable from the public surface.
 *
 * The sibling availabilitySettingsModel.test.ts covers the happy-path arms
 * (TCP method, tcp address with port, http address join, machine/service kind
 * label, Online-with-latency status). This file focuses on the remaining
 * branches of getAvailabilityTargetStatusLabel, getAvailabilityTargetKindLabel,
 * getAvailabilityTargetAddressLabel, getAvailabilityTargetMethodLabel, and
 * getAvailabilityTargetAddKind.
 */
import { describe, expect, it } from 'vitest';
import {
  getAvailabilityTargetAddKind,
  getAvailabilityTargetAddressLabel,
  getAvailabilityTargetKindLabel,
  getAvailabilityTargetMethodLabel,
  getAvailabilityTargetStatusLabel,
} from '../availabilitySettingsModel';
import type { AvailabilityProbeStatus, AvailabilityTarget } from '@/api/availabilityTargets';

// ---- Fixtures ---------------------------------------------------------------
// Mirrors the sibling availabilitySettingsModel.test.ts factory shape so the
// two files stay consistent.

const target = (overrides: Partial<AvailabilityTarget> = {}): AvailabilityTarget => ({
  id: 'mqtt-broker',
  name: 'MQTT broker',
  address: 'mqtt.local',
  protocol: 'tcp',
  port: 1883,
  enabled: true,
  ...overrides,
});

const status = (overrides: Partial<AvailabilityProbeStatus> = {}): AvailabilityProbeStatus => ({
  targetId: 'mqtt-broker',
  name: 'MQTT broker',
  address: 'mqtt.local',
  protocol: 'tcp',
  enabled: true,
  available: true,
  ...overrides,
});

// ---- getAvailabilityTargetStatusLabel --------------------------------------
// Branches: the !enabled early return ('Paused'), the !status early return
// ('Not checked yet'), the status.available truthy arm with both ternary
// operands (latency is a number vs not), and the status.available falsy arm
// with both || operands (trimmed lastError vs 'Offline' fallback).

describe('getAvailabilityTargetStatusLabel', () => {
  it("returns 'Paused' when the target is disabled (!enabled arm)", () => {
    expect(getAvailabilityTargetStatusLabel(target({ enabled: false }))).toBe('Paused');
  });

  it("returns 'Not checked yet' when enabled but status is absent (!status arm)", () => {
    expect(getAvailabilityTargetStatusLabel(target())).toBe('Not checked yet');
  });

  it("returns 'Online' without latency when latencyMillis is undefined (ternary else arm)", () => {
    expect(
      getAvailabilityTargetStatusLabel(
        target({ status: status({ available: true, latencyMillis: undefined }) }),
      ),
    ).toBe('Online');
  });

  it("returns 'Online' when latencyMillis is a non-number value (defensive typeof branch via cast)", () => {
    expect(
      getAvailabilityTargetStatusLabel(
        target({
          status: status({
            available: true,
            latencyMillis: 'fast' as unknown as number,
          }),
        }),
      ),
    ).toBe('Online');
  });

  it('returns the trimmed lastError for an unavailable target with one (|| left operand)', () => {
    expect(
      getAvailabilityTargetStatusLabel(
        target({
          status: status({ available: false, lastError: '  connection refused  ' }),
        }),
      ),
    ).toBe('connection refused');
  });

  it("returns 'Offline' for an unavailable target with no lastError (|| right operand)", () => {
    expect(
      getAvailabilityTargetStatusLabel(
        target({ status: status({ available: false, lastError: undefined }) }),
      ),
    ).toBe('Offline');
  });

  it("returns 'Offline' when lastError trims to empty (|| right operand via whitespace)", () => {
    expect(
      getAvailabilityTargetStatusLabel(
        target({ status: status({ available: false, lastError: '   ' }) }),
      ),
    ).toBe('Offline');
  });
});

// ---- getAvailabilityTargetKindLabel ----------------------------------------
// Branches: every switch arm. The sibling test already covers 'machine' and the
// undefined arm; here we cover 'device', the explicit 'service' case, and the
// default arm (only reachable via a cast, since targetKind is typed).

describe('getAvailabilityTargetKindLabel', () => {
  it("labels a device target as 'Device'", () => {
    expect(getAvailabilityTargetKindLabel(target({ targetKind: 'device' }))).toBe('Device');
  });

  it("labels an explicit service target as 'Service'", () => {
    expect(getAvailabilityTargetKindLabel(target({ targetKind: 'service' }))).toBe('Service');
  });

  it("labels an unrecognised kind as 'Endpoint' (default switch arm via cast)", () => {
    expect(
      getAvailabilityTargetKindLabel(
        target({ targetKind: 'website' as unknown as AvailabilityTarget['targetKind'] }),
      ),
    ).toBe('Endpoint');
  });
});

// ---- getAvailabilityTargetAddressLabel -------------------------------------
// Branches: the http path-join body with both ternary arms (path already starts
// with '/' vs needing one prepended), the endsWith guard (skip join), the
// path?.trim() falsy arm (undefined / whitespace), the tcp-with-port arm, and
// the final fallback arm (icmp / https / tcp-without-port).

describe('getAvailabilityTargetAddressLabel', () => {
  it('prepends a leading slash to an http path that lacks one (ternary else arm)', () => {
    expect(
      getAvailabilityTargetAddressLabel(
        target({ protocol: 'http', address: 'http://api.local', path: 'v1/health' }),
      ),
    ).toBe('http://api.local/v1/health');
  });

  it('returns the address verbatim when it already ends with the path (endsWith guard)', () => {
    expect(
      getAvailabilityTargetAddressLabel(
        target({ protocol: 'http', address: 'http://api.local/health', path: '/health' }),
      ),
    ).toBe('http://api.local/health');
  });

  it('returns the address verbatim for http when path is undefined (path falsy arm)', () => {
    expect(
      getAvailabilityTargetAddressLabel(
        target({ protocol: 'http', address: 'http://api.local', path: undefined }),
      ),
    ).toBe('http://api.local');
  });

  it('returns the address verbatim for http when path is whitespace-only (trim -> falsy arm)', () => {
    expect(
      getAvailabilityTargetAddressLabel(
        target({ protocol: 'http', address: 'http://api.local', path: '   ' }),
      ),
    ).toBe('http://api.local');
  });

  it('strips multiple trailing slashes before joining an http path', () => {
    expect(
      getAvailabilityTargetAddressLabel(
        target({ protocol: 'http', address: 'http://api.local///', path: '/health' }),
      ),
    ).toBe('http://api.local/health');
  });

  it('returns the address verbatim for tcp without a port (port falsy arm)', () => {
    expect(
      getAvailabilityTargetAddressLabel(
        target({ protocol: 'tcp', address: 'mqtt.local', port: undefined }),
      ),
    ).toBe('mqtt.local');
  });

  it('returns the address verbatim for icmp (final fallback arm)', () => {
    expect(
      getAvailabilityTargetAddressLabel(
        target({ protocol: 'icmp', address: '10.0.0.1', port: undefined }),
      ),
    ).toBe('10.0.0.1');
  });

  it('returns the address verbatim for https (final fallback arm; no https special-case)', () => {
    expect(
      getAvailabilityTargetAddressLabel(
        target({ protocol: 'https', address: 'https://api.local', path: '/health' }),
      ),
    ).toBe('https://api.local');
  });
});

// ---- getAvailabilityTargetMethodLabel --------------------------------------
// Branches: every switch arm. The sibling test covers 'icmp' and tcp-with-port;
// here we cover the tcp-without-port arm (both via undefined and 0), 'http',
// and the default arm (https plus an arbitrary protocol string via cast).

describe('getAvailabilityTargetMethodLabel', () => {
  it("returns 'TCP port' for a tcp target without a port (port falsy arm)", () => {
    expect(getAvailabilityTargetMethodLabel(target({ protocol: 'tcp', port: undefined }))).toBe(
      'TCP port',
    );
  });

  it("returns 'TCP port' for a tcp target whose port is 0 (port falsy arm via 0)", () => {
    expect(getAvailabilityTargetMethodLabel(target({ protocol: 'tcp', port: 0 }))).toBe('TCP port');
  });

  it("returns 'HTTP check' for an http target", () => {
    expect(getAvailabilityTargetMethodLabel(target({ protocol: 'http' }))).toBe('HTTP check');
  });

  it('uppercases an unrecognised but valid protocol via the default arm (https)', () => {
    expect(getAvailabilityTargetMethodLabel(target({ protocol: 'https' }))).toBe('HTTPS');
  });

  it('uppercases an arbitrary protocol string via the default arm (cast)', () => {
    expect(
      getAvailabilityTargetMethodLabel(
        target({ protocol: 'grpc' as unknown as AvailabilityTarget['protocol'] }),
      ),
    ).toBe('GRPC');
  });
});

// ---- getAvailabilityTargetAddKind ------------------------------------------
// Branches: the !shouldOpenAvailabilityTargetAddDialog guard (undefined) reached
// via a wrong pathname, a missing/wrong add param, and an unnormalisable
// targetKind; and the normalise arm returning each valid kind, plus
// case/whitespace normalisation and the trailing-slash pathname variant.

describe('getAvailabilityTargetAddKind', () => {
  it('returns undefined when the pathname is not the availability settings path (guard)', () => {
    expect(
      getAvailabilityTargetAddKind('/settings/infrastructure', '?add=target&targetKind=machine'),
    ).toBeUndefined();
  });

  it('returns undefined when the add query param is missing or wrong (guard)', () => {
    expect(
      getAvailabilityTargetAddKind('/settings/monitoring/availability', '?targetKind=machine'),
    ).toBeUndefined();
    expect(
      getAvailabilityTargetAddKind('/settings/monitoring/availability', '?add=other'),
    ).toBeUndefined();
  });

  it('returns undefined when the targetKind param does not normalise to a known kind (invalid kind guard)', () => {
    expect(
      getAvailabilityTargetAddKind(
        '/settings/monitoring/availability',
        '?add=target&targetKind=vm',
      ),
    ).toBeUndefined();
  });

  it("returns the normalised kind for 'service'", () => {
    expect(
      getAvailabilityTargetAddKind(
        '/settings/monitoring/availability',
        '?add=target&targetKind=service',
      ),
    ).toBe('service');
  });

  it("returns the normalised kind for 'device'", () => {
    expect(
      getAvailabilityTargetAddKind(
        '/settings/monitoring/availability',
        '?add=target&targetKind=device',
      ),
    ).toBe('device');
  });

  it('normalises case and surrounding whitespace on the targetKind param', () => {
    // URLSearchParams decodes '+' as space, so '+Device+' parses to ' Device '.
    expect(
      getAvailabilityTargetAddKind(
        '/settings/monitoring/availability',
        '?add=target&targetKind=+Device+',
      ),
    ).toBe('device');
  });

  it('honours the trailing-slash pathname variant', () => {
    expect(
      getAvailabilityTargetAddKind(
        '/settings/monitoring/availability/',
        '?add=target&targetKind=service',
      ),
    ).toBe('service');
  });
});
