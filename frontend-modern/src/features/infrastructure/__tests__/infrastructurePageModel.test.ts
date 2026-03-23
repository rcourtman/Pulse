import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { buildInfrastructurePageFilterDerivation } from '@/features/infrastructure/infrastructurePageModel';

const makeResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'resource-1',
  type: 'agent',
  name: 'host-1',
  displayName: 'Host 1',
  platformId: 'host-1',
  platformType: 'agent',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: 1_000,
  platformData: { sources: ['agent'] },
  tags: [],
  ...overrides,
});

describe('infrastructurePageModel', () => {
  it('builds canonical source and status filter catalogs from unified resources', () => {
    const resources = [
      makeResource({
        id: 'resource-1',
        status: 'online',
        platformData: { sources: ['agent'] },
      }),
      makeResource({
        id: 'resource-2',
        type: 'vm',
        platformType: 'proxmox-vm',
        status: 'warning',
        platformData: { sources: ['proxmox'] },
      }),
    ];

    const derivation = buildInfrastructurePageFilterDerivation(resources, '', '', '');

    expect(Array.from(derivation.availableSources)).toEqual(['agent', 'proxmox-pve']);
    expect(derivation.statusOptions).toEqual([
      { key: 'online', label: 'Online' },
      { key: 'warning', label: 'warning' },
    ]);
    expect(derivation.activeFilterCount).toBe(0);
    expect(derivation.hasActiveFilters).toBe(false);
  });

  it('filters unified resources by source, status, and search through the canonical page model', () => {
    const resources = [
      makeResource({
        id: 'resource-1',
        displayName: 'Host Alpha',
        tags: ['critical'],
        status: 'online',
        platformData: { sources: ['agent'] },
      }),
      makeResource({
        id: 'resource-2',
        displayName: 'VM Beta',
        type: 'vm',
        platformType: 'proxmox-vm',
        status: 'warning',
        platformData: { sources: ['proxmox'] },
      }),
    ];

    const derivation = buildInfrastructurePageFilterDerivation(
      resources,
      'agent',
      'online',
      'alpha critical',
    );

    expect(derivation.activeFilterCount).toBe(2);
    expect(derivation.hasActiveFilters).toBe(true);
    expect(derivation.filteredResources.map((resource) => resource.id)).toEqual(['resource-1']);
    expect(derivation.hasFilteredResources).toBe(true);
  });

  it('reports an empty filtered result without losing the active-filter signal', () => {
    const derivation = buildInfrastructurePageFilterDerivation(
      [makeResource()],
      '',
      'offline',
      '',
    );

    expect(derivation.activeFilterCount).toBe(1);
    expect(derivation.hasActiveFilters).toBe(true);
    expect(derivation.filteredResources).toEqual([]);
    expect(derivation.hasFilteredResources).toBe(false);
  });
});
