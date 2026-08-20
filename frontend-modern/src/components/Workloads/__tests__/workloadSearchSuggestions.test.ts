import { describe, expect, it } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import { buildWorkloadSearchSuggestions } from '../workloadSearchSuggestions';

describe('buildWorkloadSearchSuggestions', () => {
  it('projects workload names, ids, hosts, types, and status for every platform workload', () => {
    const workload = {
      id: 'docker:host-a:container:abc123',
      name: 'grafana-agent-01',
      displayId: 'abc123',
      type: 'container',
      status: 'running',
      dockerHostName: 'host-a',
      technology: 'docker',
    } as unknown as WorkloadGuest;

    const suggestions = buildWorkloadSearchSuggestions([workload]);
    const suggestion = suggestions.find((item) => item.id.startsWith('workload:'))!;

    expect(suggestion.label).toBe('grafana-agent-01');
    expect(suggestion.description).toContain('host-a');
    expect(suggestion.description).toContain('Running');
    expect(suggestion.keywords).toEqual(
      expect.arrayContaining(['grafana-agent-01', 'abc123', 'host-a']),
    );
    expect(suggestions).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ id: 'workload-scope:host:host-a', value: 'host-a' }),
      ]),
    );
  });
});
