import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  collectOutdatedAgentHosts,
  compareAgentVersions,
  formatAgentVersionDisplay,
  hostAgentVersion,
  parseAgentVersion,
} from './agentVersion';

describe('parseAgentVersion', () => {
  it('parses a plain semver', () => {
    expect(parseAgentVersion('6.0.0')).toEqual({ major: 6, minor: 0, patch: 0, prerelease: [] });
  });

  it('tolerates a leading v and strips build metadata', () => {
    expect(parseAgentVersion('v6.0.0-rc.6+git.151.gd8f5519ee')).toEqual({
      major: 6,
      minor: 0,
      patch: 0,
      prerelease: ['rc', '6'],
    });
  });

  it('returns null for empty or non-semver input', () => {
    expect(parseAgentVersion('')).toBeNull();
    expect(parseAgentVersion(undefined)).toBeNull();
    expect(parseAgentVersion('dev')).toBeNull();
    expect(parseAgentVersion('6.0')).toBeNull();
  });
});

describe('compareAgentVersions', () => {
  it('orders rc.5 before rc.6', () => {
    expect(compareAgentVersions('v6.0.0-rc.5', 'v6.0.0-rc.6')).toBe(-1);
    expect(compareAgentVersions('v6.0.0-rc.6', 'v6.0.0-rc.5')).toBe(1);
  });

  it('treats equal versions as equal regardless of build metadata or v prefix', () => {
    expect(compareAgentVersions('6.0.0-rc.6', 'v6.0.0-rc.6+git.151.gd8f5519ee')).toBe(0);
  });

  it('ranks a stable release above a pre-release of the same core', () => {
    expect(compareAgentVersions('6.0.0-rc.6', '6.0.0')).toBe(-1);
    expect(compareAgentVersions('6.0.0', '6.0.0-rc.6')).toBe(1);
  });

  it('orders by major, then minor, then patch', () => {
    expect(compareAgentVersions('6.0.0', '7.0.0')).toBe(-1);
    expect(compareAgentVersions('6.1.0', '6.0.9')).toBe(1);
    expect(compareAgentVersions('6.0.1', '6.0.2')).toBe(-1);
  });

  it('returns null when either side is unparseable', () => {
    expect(compareAgentVersions('dev', '6.0.0')).toBeNull();
    expect(compareAgentVersions('6.0.0', undefined)).toBeNull();
  });
});

describe('formatAgentVersionDisplay', () => {
  it('normalises to a v-prefixed string without build metadata', () => {
    expect(formatAgentVersionDisplay('6.0.0-rc.6+git.151.gd8f5519ee')).toBe('v6.0.0-rc.6');
    expect(formatAgentVersionDisplay('v6.0.0')).toBe('v6.0.0');
  });

  it('returns an empty string for unparseable input', () => {
    expect(formatAgentVersionDisplay('dev')).toBe('');
  });
});

const host = (over: Partial<Resource>): Resource => ({ id: 'x', name: 'host', type: 'docker-host', ...over }) as Resource;

describe('hostAgentVersion', () => {
  it('reads the docker meta version for a docker host', () => {
    expect(hostAgentVersion(host({ docker: { agentVersion: 'v6.0.0-rc.5' } as Resource['docker'] }))).toBe(
      'v6.0.0-rc.5',
    );
  });

  it('falls back to the agent meta version for a plain agent host', () => {
    expect(hostAgentVersion(host({ agent: { agentVersion: 'v6.0.0-rc.5' } as Resource['agent'] }))).toBe(
      'v6.0.0-rc.5',
    );
  });

  it('returns undefined when no version is reported', () => {
    expect(hostAgentVersion(host({}))).toBeUndefined();
  });
});

describe('collectOutdatedAgentHosts', () => {
  const server = 'v6.0.0-rc.6+git.151.gd8f5519ee';

  it('flags hosts strictly behind the server version', () => {
    const hosts = [
      host({ name: 'tower', docker: { agentVersion: 'v6.0.0-rc.5' } as Resource['docker'] }),
      host({ name: 'current', docker: { agentVersion: 'v6.0.0-rc.6' } as Resource['docker'] }),
      host({ name: 'delly', agent: { agentVersion: 'v6.0.0-rc.5' } as Resource['agent'] }),
    ];
    expect(collectOutdatedAgentHosts(hosts, server)).toEqual([
      { name: 'tower', version: 'v6.0.0-rc.5' },
      { name: 'delly', version: 'v6.0.0-rc.5' },
    ]);
  });

  it('does not flag hosts with no reported version', () => {
    expect(collectOutdatedAgentHosts([host({ name: 'mystery' })], server)).toEqual([]);
  });

  it('returns nothing when the server version is unknown or unparseable', () => {
    const hosts = [host({ name: 'tower', docker: { agentVersion: 'v6.0.0-rc.5' } as Resource['docker'] })];
    expect(collectOutdatedAgentHosts(hosts, '')).toEqual([]);
    expect(collectOutdatedAgentHosts(hosts, 'dev')).toEqual([]);
  });
});
