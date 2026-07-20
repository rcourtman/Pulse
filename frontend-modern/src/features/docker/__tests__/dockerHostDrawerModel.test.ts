import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';

import {
  DOCKER_HOST_DRAWER_HISTORY_GROUPS,
  getDockerHostDrawerHistoryFallbackMetrics,
  getDockerHostDrawerHistoryTarget,
} from '../dockerHostDrawerModel';

type HostOverrides = {
  agent?: { agentId?: string } | undefined;
  id?: string;
  name?: string;
  temperature?: number;
  docker?: { temperature?: number } | undefined;
};

// Minimal Resource projection for the two functions under test. id/name are
// required strings on Resource but the lookup functions must tolerate empty
// strings, so the factory defaults them to '' (which yields a null target).
const makeHost = (over: HostOverrides = {}): Resource =>
  ({
    id: over.id ?? '',
    name: over.name ?? '',
    type: 'docker-host',
    agent: over.agent,
    temperature: over.temperature,
    docker: over.docker,
  }) as unknown as Resource;

describe('dockerHostDrawerModel', () => {
  describe('getDockerHostDrawerHistoryTarget', () => {
    it('builds an agent-scoped target from the agent agentId', () => {
      expect(getDockerHostDrawerHistoryTarget(makeHost({ agent: { agentId: 'agent-9' } }))).toEqual(
        {
          resourceType: 'agent',
          resourceId: 'agent-9',
        },
      );
    });

    it('strips a leading agent: prefix from the agentId', () => {
      expect(
        getDockerHostDrawerHistoryTarget(makeHost({ agent: { agentId: 'agent:agent-9' } })),
      ).toEqual({ resourceType: 'agent', resourceId: 'agent-9' });
    });

    it('only strips the agent: prefix once', () => {
      expect(
        getDockerHostDrawerHistoryTarget(makeHost({ agent: { agentId: 'agent:agent:foo' } })),
      ).toEqual({ resourceType: 'agent', resourceId: 'agent:foo' });
    });

    it('leaves an agentId that does not start with agent: untouched', () => {
      expect(getDockerHostDrawerHistoryTarget(makeHost({ agent: { agentId: 'node-1' } }))).toEqual({
        resourceType: 'agent',
        resourceId: 'node-1',
      });
    });

    it('trims surrounding whitespace before resolving the id', () => {
      expect(
        getDockerHostDrawerHistoryTarget(makeHost({ agent: { agentId: '  agent:foo  ' } })),
      ).toEqual({ resourceType: 'agent', resourceId: 'foo' });
    });

    it('falls back to the resource id when no agentId is reported', () => {
      expect(getDockerHostDrawerHistoryTarget(makeHost({ id: 'host-1' }))).toEqual({
        resourceType: 'agent',
        resourceId: 'host-1',
      });
    });

    it('strips the agent: prefix from the id fallback too', () => {
      expect(getDockerHostDrawerHistoryTarget(makeHost({ id: 'agent:host-1' }))).toEqual({
        resourceType: 'agent',
        resourceId: 'host-1',
      });
    });

    it('falls back to the resource name when neither agentId nor id are present', () => {
      expect(getDockerHostDrawerHistoryTarget(makeHost({ name: 'edge-01' }))).toEqual({
        resourceType: 'agent',
        resourceId: 'edge-01',
      });
    });

    it('prefers agentId over id, and id over name', () => {
      expect(
        getDockerHostDrawerHistoryTarget(
          makeHost({ agent: { agentId: 'agent:agent-1' }, id: 'id-1', name: 'named' }),
        )?.resourceId,
      ).toBe('agent-1');
      expect(
        getDockerHostDrawerHistoryTarget(makeHost({ id: 'id-1', name: 'named' }))?.resourceId,
      ).toBe('id-1');
    });

    it('treats an empty agentId string as absent and falls through to the id', () => {
      expect(
        getDockerHostDrawerHistoryTarget(makeHost({ agent: { agentId: '' }, id: 'id-1' }))
          ?.resourceId,
      ).toBe('id-1');
    });

    it('always reports the agent resourceType when a target is resolved', () => {
      const target = getDockerHostDrawerHistoryTarget(makeHost({ name: 'edge-01' }));
      expect(target?.resourceType).toBe('agent');
    });

    it('returns null when every candidate is empty or missing', () => {
      expect(getDockerHostDrawerHistoryTarget(makeHost())).toBeNull();
    });

    it('returns null when the only candidate is whitespace', () => {
      expect(getDockerHostDrawerHistoryTarget(makeHost({ id: '   ', name: '   ' }))).toBeNull();
    });
  });

  describe('getDockerHostDrawerHistoryFallbackMetrics', () => {
    it('returns the host temperature when it is a finite number', () => {
      expect(getDockerHostDrawerHistoryFallbackMetrics(makeHost({ temperature: 42 }))).toEqual({
        temperature: 42,
      });
    });

    it('falls back to docker.temperature when host temperature is absent', () => {
      expect(
        getDockerHostDrawerHistoryFallbackMetrics(makeHost({ docker: { temperature: 51 } })),
      ).toEqual({ temperature: 51 });
    });

    it('prefers host temperature over docker.temperature', () => {
      expect(
        getDockerHostDrawerHistoryFallbackMetrics(
          makeHost({ temperature: 10, docker: { temperature: 99 } }),
        ),
      ).toEqual({ temperature: 10 });
    });

    it('keeps zero as a valid temperature (boundary)', () => {
      expect(getDockerHostDrawerHistoryFallbackMetrics(makeHost({ temperature: 0 }))).toEqual({
        temperature: 0,
      });
    });

    it('keeps a finite negative temperature', () => {
      expect(getDockerHostDrawerHistoryFallbackMetrics(makeHost({ temperature: -5 }))).toEqual({
        temperature: -5,
      });
    });

    it('rejects NaN on the host temperature and falls through to docker', () => {
      expect(
        getDockerHostDrawerHistoryFallbackMetrics(
          makeHost({ temperature: Number.NaN, docker: { temperature: 33 } }),
        ),
      ).toEqual({ temperature: 33 });
    });

    it('rejects Infinity and -Infinity on the host temperature', () => {
      expect(
        getDockerHostDrawerHistoryFallbackMetrics(
          makeHost({ temperature: Number.POSITIVE_INFINITY }),
        ),
      ).toEqual({ temperature: undefined });
      expect(
        getDockerHostDrawerHistoryFallbackMetrics(
          makeHost({ temperature: Number.NEGATIVE_INFINITY }),
        ),
      ).toEqual({ temperature: undefined });
    });

    it('returns undefined when both host and docker temperatures are non-finite', () => {
      expect(
        getDockerHostDrawerHistoryFallbackMetrics(
          makeHost({ temperature: Number.NaN, docker: { temperature: Number.POSITIVE_INFINITY } }),
        ),
      ).toEqual({ temperature: undefined });
    });

    it('returns undefined when neither temperature source is present', () => {
      expect(getDockerHostDrawerHistoryFallbackMetrics(makeHost())).toEqual({
        temperature: undefined,
      });
    });

    it('always returns exactly one temperature key', () => {
      const metrics = getDockerHostDrawerHistoryFallbackMetrics(makeHost({ temperature: 7 }));
      expect(Object.keys(metrics)).toEqual(['temperature']);
    });
  });

  describe('DOCKER_HOST_DRAWER_HISTORY_GROUPS', () => {
    it('declares the four operator chart groups for a Docker host', () => {
      expect(DOCKER_HOST_DRAWER_HISTORY_GROUPS.map((group) => group.id)).toEqual([
        'utilization',
        'network',
        'disk-io',
        'thermals',
      ]);
    });

    it.each([
      ['utilization', 'Utilization', '%'],
      ['network', 'Network I/O', 'B/s'],
      ['disk-io', 'Disk I/O', 'B/s'],
      ['thermals', 'Thermals', 'C'],
    ])('group %s has label %s and unit %s', (id, label, unit) => {
      const group = DOCKER_HOST_DRAWER_HISTORY_GROUPS.find((entry) => entry.id === id);
      expect(group?.label).toBe(label);
      expect(group?.unit).toBe(unit);
    });

    it('declares the utilization cpu/memory/disk series', () => {
      const utilization = DOCKER_HOST_DRAWER_HISTORY_GROUPS.find((g) => g.id === 'utilization');
      expect(utilization?.series.map((series) => series.metric)).toEqual(['cpu', 'memory', 'disk']);
      expect(utilization?.series.every((series) => series.unit === '%')).toBe(true);
    });

    it('declares the network in/out series', () => {
      const network = DOCKER_HOST_DRAWER_HISTORY_GROUPS.find((g) => g.id === 'network');
      expect(network?.series.map((series) => series.metric)).toEqual(['netin', 'netout']);
    });

    it('declares the disk-io read/write series', () => {
      const diskIo = DOCKER_HOST_DRAWER_HISTORY_GROUPS.find((g) => g.id === 'disk-io');
      expect(diskIo?.series.map((series) => series.metric)).toEqual(['diskread', 'diskwrite']);
    });

    it('declares a single temperature series in the thermals group', () => {
      const thermals = DOCKER_HOST_DRAWER_HISTORY_GROUPS.find((g) => g.id === 'thermals');
      expect(thermals?.series.map((series) => series.metric)).toEqual(['temperature']);
      expect(thermals?.series[0].label).toBe('CPU');
      expect(thermals?.series[0].unit).toBe('C');
    });

    it('gives every series a label, unit, and a hex color', () => {
      for (const group of DOCKER_HOST_DRAWER_HISTORY_GROUPS) {
        for (const series of group.series) {
          expect(series.label.length).toBeGreaterThan(0);
          expect(series.unit.length).toBeGreaterThan(0);
          expect(series.color).toMatch(/^#[0-9a-f]{6}$/i);
        }
      }
    });

    it('exposes a temperature metric that matches the fallback metrics key', () => {
      // The drawer fallback can only synthesize a flat line for metrics it
      // declares a current value for; the only such metric for a Docker host
      // is temperature, which must exist as a series metric here.
      const thermals = DOCKER_HOST_DRAWER_HISTORY_GROUPS.find((g) => g.id === 'thermals');
      expect(thermals?.series.some((series) => series.metric === 'temperature')).toBe(true);
    });
  });
});
