import { describe, expect, it, vi } from 'vitest';
import { render } from '@solidjs/testing-library';
import type { Resource } from '@/types/resource';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';

if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    isMobile: () => false,
    isVisible: () => true,
  }),
}));

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: () => <div data-testid="metric-cell">metric</div>,
}));

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: () => <div data-testid="resource-drawer">drawer</div>,
}));

const resource: Resource = {
  id: 'resource-1',
  type: 'agent',
  name: 'sensitive-host',
  displayName: 'Sensitive Host',
  platformId: 'platform-1',
  platformType: 'agent',
  sourceType: 'agent',
  status: 'online',
  lastSeen: Date.now(),
  platformData: { sources: ['agent'] },
  policy: {
    sensitivity: 'restricted',
    routing: {
      scope: 'local-only',
      redact: ['hostname', 'alias'],
    },
  },
  aiSafeSummary: 'restricted host summary safe for remote AI consumption',
  recentChanges: [
    {
      id: 'change-1',
      observedAt: new Date().toISOString(),
      resourceId: 'resource-1',
      kind: 'state_transition',
      sourceType: 'pulse_diff',
      confidence: 'high',
    },
  ],
};

describe('UnifiedResourceTable governance presentation', () => {
  it('surfaces canonical policy badges in the resource row', () => {
    const { getByText, queryByText } = render(() => (
      <UnifiedResourceTable
        resources={[resource]}
        expandedResourceId={null}
        onExpandedResourceChange={vi.fn()}
        groupingMode="flat"
      />
    ));

    expect(getByText('Restricted')).toBeInTheDocument();
    expect(getByText('Local Only')).toBeInTheDocument();
    expect(getByText('Sensitive Host')).toBeInTheDocument();
    expect(queryByText('restricted host summary safe for remote AI consumption')).toBeNull();
    expect(queryByText('(sensitive-host)')).toBeNull();
  });

  it('surfaces resource facet counts in the resource row', () => {
    const { getByText, queryByText } = render(() => (
      <UnifiedResourceTable
        resources={[resource]}
        expandedResourceId={null}
        onExpandedResourceChange={vi.fn()}
        groupingMode="flat"
      />
    ));

    expect(getByText('Timeline 1')).toBeInTheDocument();
    expect(queryByText('Capabilities 2')).toBeNull();
    expect(queryByText('Relationships 1')).toBeNull();
  });
});
