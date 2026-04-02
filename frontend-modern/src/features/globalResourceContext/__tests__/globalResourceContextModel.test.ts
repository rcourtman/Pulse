import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  areGlobalResourceContextPathsEquivalent,
  buildGlobalResourceContextBasePath,
  buildGlobalResourceContextScopedPath,
  buildSurfacePathWithGlobalResourceContext,
  resolveGlobalResourceContextClearTarget,
  resolveGlobalResourceContextSurface,
} from '@/features/globalResourceContext/globalResourceContextModel';

const makeAgent = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'pve-1',
    type: 'agent',
    name: 'pve1',
    displayName: 'pve1',
    platformId: 'pve-1',
    platformType: 'proxmox-pve',
    sourceType: 'agent',
    status: 'online',
    lastSeen: Date.now(),
    ...overrides,
  }) as Resource;

describe('globalResourceContextModel', () => {
  it('resolves canonical surfaces from platform paths', () => {
    expect(resolveGlobalResourceContextSurface('/dashboard')).toBe('dashboard');
    expect(resolveGlobalResourceContextSurface('/infrastructure')).toBe('infrastructure');
    expect(resolveGlobalResourceContextSurface('/workloads')).toBe('workloads');
    expect(resolveGlobalResourceContextSurface('/storage')).toBe('storage');
    expect(resolveGlobalResourceContextSurface('/ceph')).toBe('storage');
    expect(resolveGlobalResourceContextSurface('/recovery')).toBe('recovery');
    expect(resolveGlobalResourceContextSurface('/settings')).toBeNull();
  });

  it('builds scoped platform paths from a canonical pinned resource', () => {
    const resource = makeAgent();
    expect(buildGlobalResourceContextBasePath('storage')).toBe('/storage');
    expect(buildGlobalResourceContextScopedPath('infrastructure', resource)).toBe(
      '/infrastructure?resource=pve-1',
    );
    expect(buildGlobalResourceContextScopedPath('workloads', resource)).toBe(
      '/workloads?agent=pve1',
    );
    expect(buildGlobalResourceContextScopedPath('storage', resource)).toBe(
      '/storage?source=proxmox-pve&node=pve-1',
    );
    expect(buildGlobalResourceContextScopedPath('recovery', resource)).toBe(
      '/recovery?platform=proxmox-pve&node=pve-1',
    );
  });

  it('adds the canonical global context query param to scoped surface routes', () => {
    expect(buildSurfacePathWithGlobalResourceContext('dashboard', makeAgent())).toBe(
      '/dashboard?contextResource=pve-1',
    );
    expect(buildSurfacePathWithGlobalResourceContext('storage', makeAgent())).toBe(
      '/storage?source=proxmox-pve&node=pve-1&contextResource=pve-1',
    );
  });

  it('treats paths with equivalent local filters as equal when only query ordering differs', () => {
    expect(
      areGlobalResourceContextPathsEquivalent(
        '/storage?source=proxmox-pve&node=pve-1&contextResource=pve-1',
        '/storage?node=pve-1&contextResource=pve-1&source=proxmox-pve',
      ),
    ).toBe(true);
  });

  it('clears back to the base platform path when the current route is only context-derived', () => {
    const target = resolveGlobalResourceContextClearTarget({
      currentPath: '/storage?source=proxmox-pve&node=pve-1&contextResource=pve-1',
      resource: makeAgent(),
    });
    expect(target).toBe('/storage');
  });

  it('only strips the context param when the page has diverged from the derived scoped path', () => {
    const target = resolveGlobalResourceContextClearTarget({
      currentPath: '/storage?source=proxmox-pve&node=pve-1&q=tank&contextResource=pve-1',
      resource: makeAgent(),
    });
    expect(target).toBe('/storage?source=proxmox-pve&node=pve-1&q=tank');
  });
});
