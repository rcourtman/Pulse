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
});
