import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';

import { ResourceChangeSummary } from '../ResourceChangeSummary';

describe('ResourceChangeSummary', () => {
  it('sorts recent changes canonically and truncates the feed', () => {
    render(() => (
      <ResourceChangeSummary
        class="mt-2"
        title="Recent changes"
        subtitle="Canonical 24h timeline"
        changes={[
          {
            id: 'older-change',
            resourceId: 'storage-1',
            kind: 'config_update',
            reason: 'Updated retention policy',
            observedAt: '2026-03-18T10:00:00Z',
            sourceType: 'pulse_diff',
            confidence: 'medium',
            actor: 'agent:ops-helper',
            relatedResources: ['vm-100'],
          },
          {
            id: 'newer-change',
            resourceId: 'storage-2',
            kind: 'restart',
            from: 'running',
            to: 'restarting',
            observedAt: '2026-03-18T12:00:00Z',
            sourceType: 'platform_event',
            sourceAdapter: 'docker_adapter',
            confidence: 'high',
            actor: 'agent:ops-helper',
            relatedResources: ['vm-200', 'vm-300'],
          },
        ]}
        buildResourceHref={(resourceId) => `/infrastructure?resource=${resourceId}`}
        maxChanges={1}
      />
    ));

    expect(screen.getByText('Recent changes')).toBeInTheDocument();
    expect(screen.getByText('Canonical 24h timeline')).toBeInTheDocument();
    expect(screen.getByText('Restart: running → restarting')).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: 'Open resource storage-2 in Infrastructure' }),
    ).toHaveAttribute('href', '/infrastructure?resource=storage-2');
    expect(
      screen.getByRole('link', { name: 'Open related resource vm-200 in Infrastructure' }),
    ).toHaveAttribute('href', '/infrastructure?resource=vm-200');
    expect(screen.getByText('By agent:ops-helper')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Open resource storage-1 in Infrastructure' })).toBeNull();
  });

  it('renders the empty state when no recent changes are available', () => {
    render(() => (
      <ResourceChangeSummary
        title="Latest canonical change"
        emptyText="No canonical changes were recorded."
        changes={[]}
        buildResourceHref={(resourceId) => `/infrastructure?resource=${resourceId}`}
      />
    ));

    expect(screen.getByText('Latest canonical change')).toBeInTheDocument();
    expect(screen.getByText('No canonical changes were recorded.')).toBeInTheDocument();
  });
});
