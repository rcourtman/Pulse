import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

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
  lastSeen: () => '14 mins ago',
  lastSeenAbsolute: () => '',
} as unknown as UseResourceDetailDrawerStateResult;

const machine = (status: Resource['status']): Resource => ({
  id: 'agent-apollo',
  name: 'Apollo-114',
  displayName: 'Apollo-114',
  platformId: 'host-linux-1',
  platformType: 'agent',
  sourceType: 'agent',
  type: 'agent',
  status,
  uptime: 2_500_000,
  lastSeen: Date.parse('2026-10-06T22:00:00Z'),
});

afterEach(cleanup);

describe('resource drawer runtime context for an offline resource', () => {
  it('drops the uptime of an offline machine and keeps when it was last seen', () => {
    render(() => (
      <InlineResourceSummaryTables
        resource={machine('offline')}
        drawer={drawerState}
        showPlatformId={false}
      />
    ));

    const section = screen.getByTestId('resource-runtime-context-section');
    expect(within(section).getByText('offline')).toBeInTheDocument();
    expect(within(section).queryByText('Uptime')).toBeNull();
    expect(within(section).getByText('Last seen')).toBeInTheDocument();
    expect(within(section).getByText('14 mins ago')).toBeInTheDocument();
  });

  it('keeps the uptime of a machine that is up', () => {
    render(() => (
      <InlineResourceSummaryTables
        resource={machine('online')}
        drawer={drawerState}
        showPlatformId={false}
      />
    ));

    const section = screen.getByTestId('resource-runtime-context-section');
    expect(within(section).getByText('Uptime')).toBeInTheDocument();
  });
});
