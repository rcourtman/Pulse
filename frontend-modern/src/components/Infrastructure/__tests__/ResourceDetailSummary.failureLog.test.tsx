import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { InlineResourceSummaryTables } from '../ResourceDetailSummary';
import type { UseResourceDetailDrawerStateResult } from '../useResourceDetailDrawerState';

const drawerState = {
  primaryIdentityRows: () => [],
  identityIpValues: () => [],
  identityAliasValues: () => [],
  aliasPreviewValues: () => [],
  hasAliasOverflow: () => false,
  identityCardHasRichData: () => true,
  sourceSummary: () => null,
  lastSeen: () => '',
  lastSeenAbsolute: () => '',
} as unknown as UseResourceDetailDrawerStateResult;

const container = (docker: NonNullable<Resource['docker']>): Resource =>
  ({
    id: 'app-container-1',
    name: 'vaultwarden',
    displayName: 'vaultwarden',
    platformId: 'host-1',
    platformType: 'docker',
    sourceType: 'agent',
    status: 'offline',
    type: 'app-container',
    lastSeen: Date.parse('2026-11-13T02:05:00Z'),
    docker,
  }) as Resource;

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('resource drawer failure log', () => {
  it('shows the last log lines of a failed container as text, with where they came from', () => {
    vi.useFakeTimers({ now: new Date('2026-11-13T02:05:00Z') });
    render(() => (
      <InlineResourceSummaryTables
        resource={container({
          containerState: 'exited',
          exitCode: 1,
          failureLog: {
            reason: 'exited',
            capturedAt: '2026-11-13T02:02:00Z',
            truncated: true,
            lines: [
              '[vaultwarden::db][ERROR] invalid DATABASE_URL "[REDACTED]"',
              '<img src=x onerror=alert(1)> is only text',
            ],
          },
        })}
        drawer={drawerState}
        showPlatformId={false}
      />
    ));

    const section = screen.getByTestId('resource-drawer-card-docker-failure-log');
    expect(section.textContent).toContain('Last log lines');
    expect(section.textContent).toContain('Captured 3 mins ago');
    expect(section.textContent).toContain('Secrets masked on the host');
    expect(section.textContent).toContain('Cut to the last lines');

    const lines = screen.getByTestId('resource-failure-log-lines');
    expect(lines.tagName).toBe('PRE');
    expect(lines.textContent).toBe(
      '[vaultwarden::db][ERROR] invalid DATABASE_URL "[REDACTED]"\n<img src=x onerror=alert(1)> is only text',
    );
    // A log line is whatever the container printed: it must not become markup.
    expect(lines.querySelector('img')).toBeNull();
    expect(lines.children.length).toBe(0);
  });

  it('shows no section for a container that reports no lines', () => {
    render(() => (
      <InlineResourceSummaryTables
        resource={container({ containerState: 'exited', exitCode: 1 })}
        drawer={drawerState}
        showPlatformId={false}
      />
    ));
    expect(screen.queryByTestId('resource-drawer-card-docker-failure-log')).toBeNull();

    cleanup();
    render(() => (
      <InlineResourceSummaryTables
        resource={container({
          containerState: 'exited',
          exitCode: 1,
          failureLog: { reason: 'exited', capturedAt: '2026-11-13T02:02:00Z', lines: [] },
        })}
        drawer={drawerState}
        showPlatformId={false}
      />
    ));
    expect(screen.queryByTestId('resource-drawer-card-docker-failure-log')).toBeNull();
  });
});
