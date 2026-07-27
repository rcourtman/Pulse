import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { collectHostIdentityConflictHosts } from '../hostIdentityConflict';
import { HostIdentityConflictNotice } from '../HostIdentityConflictNotice';

const agentHost = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'agent-host-1',
    type: 'agent',
    name: 'agent-host-1',
    ...overrides,
  }) as Resource;

describe('collectHostIdentityConflictHosts', () => {
  it('returns an empty list when no host carries a conflict', () => {
    expect(
      collectHostIdentityConflictHosts([agentHost(), agentHost({ id: 'h2', name: 'h2' })]),
    ).toEqual([]);
  });

  it('collects conflicting hosts with their hostnames and report IPs', () => {
    const hosts = [
      agentHost({
        agent: {
          identityConflict: {
            hostnames: ['clone-a', ' clone-b '],
            reportIps: ['192.168.1.10', ' 10.0.0.10 '],
            firstSeen: '2026-07-27T12:00:00Z',
            lastSeen: '2026-07-27T12:05:00Z',
          },
        },
      }),
      agentHost({ id: 'healthy', name: 'healthy' }),
    ];
    expect(collectHostIdentityConflictHosts(hosts)).toEqual([
      {
        name: 'agent-host-1',
        hostnames: ['clone-a', 'clone-b'],
        reportIps: ['192.168.1.10', '10.0.0.10'],
      },
    ]);
  });

  it('falls back to the resource id when the name is missing', () => {
    const hosts = [agentHost({ name: '', agent: { identityConflict: { hostnames: ['a', 'b'] } } })];
    expect(collectHostIdentityConflictHosts(hosts)[0].name).toBe('agent-host-1');
  });
});

describe('HostIdentityConflictNotice', () => {
  afterEach(() => cleanup());

  it('renders nothing without conflicts', () => {
    render(() => <HostIdentityConflictNotice hosts={[]} />);
    expect(screen.queryByTestId('host-identity-conflict-notice')).toBeNull();
  });

  it('explains a single conflicted host with its flapping hostnames', () => {
    render(() => (
      <HostIdentityConflictNotice
        hosts={[{ name: 'agent-host-1', hostnames: ['clone-a', 'clone-b'], reportIps: [] }]}
      />
    ));
    const notice = screen.getByTestId('host-identity-conflict-notice');
    expect(notice.textContent).toContain(
      'Two machines appear to share the identity of agent-host-1',
    );
    expect(notice.textContent).toContain('clone-a, clone-b');
    expect(notice.textContent).toContain('/etc/machine-id');
  });

  it('falls back to report IPs when the clones reuse one hostname', () => {
    render(() => (
      <HostIdentityConflictNotice
        hosts={[
          {
            name: 'pve01',
            hostnames: ['pve01'],
            reportIps: ['192.168.1.10', '10.0.0.10'],
          },
        ]}
      />
    ));
    const notice = screen.getByTestId('host-identity-conflict-notice');
    expect(notice.textContent).toContain('Two machines appear to share the identity of pve01');
    expect(notice.textContent).toContain('192.168.1.10, 10.0.0.10');
  });

  it('lists every affected host when several conflict', () => {
    render(() => (
      <HostIdentityConflictNotice
        hosts={[
          { name: 'host-1', hostnames: ['a', 'b'], reportIps: [] },
          { name: 'host-2', hostnames: ['host-2'], reportIps: ['1.1.1.1', '2.2.2.2'] },
        ]}
      />
    ));
    const notice = screen.getByTestId('host-identity-conflict-notice');
    expect(notice.textContent).toContain('2 hosts');
    expect(notice.textContent).toContain('host-1 (a, b)');
    expect(notice.textContent).toContain('host-2 (1.1.1.1, 2.2.2.2)');
  });
});
