import { Route, Router } from '@solidjs/router';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { PlatformEstateOverview } from '../PlatformEstateOverview';
import { PLATFORM_ESTATE_OVERVIEW_STORAGE_KEY } from '../platformEstateOverviewModel';

const resources = [
  {
    id: 'pve-a',
    type: 'agent',
    name: 'pve-a',
    displayName: 'pve-a',
    platformId: 'proxmox',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'offline',
    lastSeen: 1_700_000_000_000,
    proxmox: { clusterName: 'production' },
  },
  {
    id: 'vm-101',
    type: 'vm',
    name: 'database',
    displayName: 'Database',
    platformId: 'proxmox',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'running',
    lastSeen: 1_700_000_000_000,
  },
] as Resource[];

const renderOverview = () =>
  render(() => (
    <Router>
      <Route
        path="/"
        component={() => <PlatformEstateOverview platform="proxmox" resources={resources} />}
      />
    </Router>
  ));

beforeEach(() => {
  window.localStorage.clear();
});

afterEach(() => {
  cleanup();
  window.localStorage.clear();
});

describe('PlatformEstateOverview', () => {
  it('renders the shared metrics and operational spotlights by default', () => {
    renderOverview();

    expect(screen.getByRole('heading', { name: 'Estate at a glance' })).toBeInTheDocument();
    expect(screen.getByText('Proxmox nodes')).toBeInTheDocument();
    expect(screen.getByText('VMs and containers')).toBeInTheDocument();
    expect(screen.getByText('pve-a offline')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /pve-a offline/i })).toHaveAttribute(
      'href',
      '/proxmox/overview',
    );
  });

  it('persists the global hide and show choice', async () => {
    renderOverview();

    await fireEvent.click(screen.getByRole('button', { name: 'Hide' }));
    expect(screen.queryByRole('heading', { name: 'Estate at a glance' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Show estate overview' })).toBeInTheDocument();
    expect(window.localStorage.getItem(PLATFORM_ESTATE_OVERVIEW_STORAGE_KEY)).toBe('false');

    await fireEvent.click(screen.getByRole('button', { name: 'Show estate overview' }));
    expect(screen.getByRole('heading', { name: 'Estate at a glance' })).toBeInTheDocument();
    expect(window.localStorage.getItem(PLATFORM_ESTATE_OVERVIEW_STORAGE_KEY)).toBe('true');
  });

  it('honors an existing hidden preference before first paint', () => {
    window.localStorage.setItem(PLATFORM_ESTATE_OVERVIEW_STORAGE_KEY, 'false');

    renderOverview();

    expect(screen.queryByRole('heading', { name: 'Estate at a glance' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Show estate overview' })).toBeInTheDocument();
  });
});
