import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { collectIdentityConflictHosts } from '../dockerIdentityConflict';
import { DockerIdentityConflictNotice } from '../DockerIdentityConflictNotice';

const dockerHost = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'docker-host-1',
    type: 'docker-host',
    name: 'docker-host-1',
    ...overrides,
  }) as Resource;

describe('collectIdentityConflictHosts', () => {
  it('returns an empty list when no host carries a conflict', () => {
    expect(
      collectIdentityConflictHosts([dockerHost(), dockerHost({ id: 'h2', name: 'h2' })]),
    ).toEqual([]);
  });

  it('collects conflicting hosts with their hostnames', () => {
    const hosts = [
      dockerHost({
        docker: {
          identityConflict: {
            hostnames: ['clone-a', ' clone-b '],
            firstSeen: '2026-07-16T12:00:00Z',
            lastSeen: '2026-07-16T12:05:00Z',
          },
        },
      }),
      dockerHost({ id: 'healthy', name: 'healthy' }),
    ];
    expect(collectIdentityConflictHosts(hosts)).toEqual([
      { name: 'docker-host-1', hostnames: ['clone-a', 'clone-b'] },
    ]);
  });

  it('falls back to the resource id when the name is missing', () => {
    const hosts = [
      dockerHost({ name: '', docker: { identityConflict: { hostnames: ['a', 'b'] } } }),
    ];
    expect(collectIdentityConflictHosts(hosts)[0].name).toBe('docker-host-1');
  });
});

describe('DockerIdentityConflictNotice', () => {
  afterEach(() => cleanup());

  it('renders nothing without conflicts', () => {
    render(() => <DockerIdentityConflictNotice hosts={[]} />);
    expect(screen.queryByTestId('docker-identity-conflict-notice')).toBeNull();
  });

  it('explains a single conflicted host with its flapping hostnames', () => {
    render(() => (
      <DockerIdentityConflictNotice
        hosts={[{ name: 'docker-host-1', hostnames: ['clone-a', 'clone-b'] }]}
      />
    ));
    const notice = screen.getByTestId('docker-identity-conflict-notice');
    expect(notice.textContent).toContain(
      'Two machines appear to share the identity of docker-host-1',
    );
    expect(notice.textContent).toContain('clone-a, clone-b');
    expect(notice.textContent).toContain('/etc/machine-id');
  });

  it('lists every affected host when several conflict', () => {
    render(() => (
      <DockerIdentityConflictNotice
        hosts={[
          { name: 'host-1', hostnames: ['a', 'b'] },
          { name: 'host-2', hostnames: ['c', 'd'] },
        ]}
      />
    ));
    const notice = screen.getByTestId('docker-identity-conflict-notice');
    expect(notice.textContent).toContain('2 hosts');
    expect(notice.textContent).toContain('host-1 (a, b)');
    expect(notice.textContent).toContain('host-2 (c, d)');
  });
});
