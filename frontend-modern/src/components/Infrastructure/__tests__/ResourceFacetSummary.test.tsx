import { describe, expect, it } from 'vitest';
import { render } from '@solidjs/testing-library';
import { ResourceFacetSummary } from '@/components/Infrastructure/ResourceFacetSummary';

describe('ResourceFacetSummary', () => {
  it('renders timeline-only badges for canonical resource counts', () => {
    const { getByText, queryByText } = render(() => (
      <ResourceFacetSummary
        counts={{
          recentChanges: 3,
          recentChangeKinds: {
            restart: 2,
            config_update: 1,
            metric_anomaly: 1,
          },
          recentChangeSourceTypes: {
            platform_event: 1,
            pulse_diff: 2,
          },
          recentChangeSourceAdapters: {
            docker_adapter: 2,
            proxmox_adapter: 1,
          },
        }}
        recentChanges={[]}
      />
    ));

    expect(getByText('Timeline 3')).toBeInTheDocument();
    expect(getByText('Restart 2')).toBeInTheDocument();
    expect(getByText('Config update 1')).toBeInTheDocument();
    expect(getByText('Anomaly 1')).toBeInTheDocument();
    expect(getByText('Platform event 1')).toBeInTheDocument();
    expect(getByText('Pulse diff 2')).toBeInTheDocument();
    expect(getByText('Docker adapter 2')).toBeInTheDocument();
    expect(getByText('Proxmox adapter 1')).toBeInTheDocument();
    expect(queryByText('Capabilities 1')).toBeNull();
    expect(queryByText('Relationships 1')).toBeNull();
  });
});
