import { describe, expect, it } from 'vitest';
import { render, screen } from '@solidjs/testing-library';
import { ProblemResourcesTable } from '../ProblemResourcesTable';
import type { ProblemResource } from '@/hooks/useDashboardOverview';
import type { Resource } from '@/types/resource';

function makeResource(overrides: Partial<Resource> = {}): Resource {
  return {
    id: 'test-1',
    type: 'agent',
    name: 'test-node',
    displayName: 'Test Node',
    platformId: 'pve-1',
    platformType: 'proxmox',
    sourceType: 'proxmox',
    status: 'offline',
    ...overrides,
  } as Resource;
}

function makeProblems(count: number): ProblemResource[] {
  return Array.from({ length: count }, (_, i) => ({
    resource: makeResource({ id: `res-${i}`, name: `Resource ${i}`, displayName: `Resource ${i}` }),
    problems: ['Offline'],
    worstValue: 200,
  }));
}

describe('ProblemResourcesTable', () => {
  it('does not render overflow footer when fewer than 8 problems', () => {
    render(() => <ProblemResourcesTable problems={makeProblems(7)} />);
    // Footer links should not appear
    expect(screen.queryByText('Infrastructure')).toBeNull();
    expect(screen.queryByText('Workloads')).toBeNull();
    expect(screen.queryByText('Storage')).toBeNull();
  });

  it('renders overflow footer with Infrastructure, Workloads, and Storage links when 8+ problems', () => {
    render(() => <ProblemResourcesTable problems={makeProblems(8)} />);

    const resourceLink = screen.getByText('Resource 0');
    expect(resourceLink.tagName).toBe('A');
    expect(resourceLink.getAttribute('href')).toBe('/infrastructure?resource=res-0');

    const infraLink = screen.getByText('Infrastructure');
    expect(infraLink.tagName).toBe('A');
    expect(infraLink.getAttribute('href')).toBe('/infrastructure');

    const workloadsLink = screen.getByText('Workloads');
    expect(workloadsLink.tagName).toBe('A');
    expect(workloadsLink.getAttribute('href')).toBe('/workloads');

    const storageLink = screen.getByText('Storage');
    expect(storageLink.tagName).toBe('A');
    expect(storageLink.getAttribute('href')).toBe('/storage');
  });

  it('renders governed labels for policy-aware problem resources', () => {
    render(() => (
      <ProblemResourcesTable
        problems={[
          {
            resource: makeResource({
              id: 'governed-1',
              name: 'sensitive-host',
              displayName: 'Sensitive Host',
              aiSafeSummary: 'restricted host summary safe for remote AI consumption',
              policy: {
                sensitivity: 'restricted',
                routing: {
                  scope: 'local-only',
                  redact: ['hostname', 'alias'],
                },
              },
            }),
            problems: ['Offline'],
            worstValue: 200,
          },
        ]}
      />
    ));

    const resourceLink = screen.getByText('restricted host summary safe for remote AI consumption');
    expect(resourceLink.tagName).toBe('A');
    expect(resourceLink.getAttribute('title')).toBe(
      'restricted host summary safe for remote AI consumption',
    );
    expect(screen.queryByText('Sensitive Host')).toBeNull();
    expect(screen.queryByText('sensitive-host')).toBeNull();
  });
});
