import { describe, expect, it } from 'vitest';
import {
  getWorkloadsDisconnectedBannerState,
  getWorkloadsDisconnectedState,
  getWorkloadsGuestsEmptyState,
  getWorkloadsInfrastructureEmptyState,
  getWorkloadsLoadingState,
  getWorkloadsNoResourcesState,
  getWorkloadsNoInventoryState,
  getWorkloadsUnavailableState,
} from '@/utils/workloadEmptyStatePresentation';

describe('workloadEmptyStatePresentation', () => {
  it('returns the infrastructure onboarding empty state', () => {
    const state = getWorkloadsInfrastructureEmptyState();

    expect(state).toEqual({
      title: 'No infrastructure sources connected',
      description:
        'Start in Settings → Infrastructure by choosing a source strategy. Connect a platform API for inventory and health, install Pulse Agent for host telemetry, or use both when you want full coverage.',
      actionLabel: 'Add infrastructure source',
    });
    expect(state.description).not.toContain('Settings → Infrastructure → Proxmox');
  });

  it('returns the guest filter empty state for empty search', () => {
    expect(getWorkloadsGuestsEmptyState('')).toEqual({
      title: 'No guests found',
      description: 'No guests match your current filters',
    });
  });

  it('returns the guest filter empty state for a search term', () => {
    expect(getWorkloadsGuestsEmptyState('  proxmox  ')).toEqual({
      title: 'No guests found',
      description: 'No guests match your search "proxmox"',
    });
  });

  it('returns the workloads loading-state copy', () => {
    expect(getWorkloadsLoadingState(false)).toEqual({
      title: 'Loading workloads...',
      description: 'Connecting to monitoring service',
    });
    expect(getWorkloadsLoadingState(true)).toEqual({
      title: 'Loading workloads...',
      description: 'Reconnecting to monitoring service…',
    });
  });

  it('returns the workloads disconnected-state copy', () => {
    expect(getWorkloadsDisconnectedState(true)).toEqual({
      title: 'Connection lost',
      description: 'Attempting to reconnect…',
      actionLabel: undefined,
    });
    expect(getWorkloadsDisconnectedState(false)).toEqual({
      title: 'Connection lost',
      description: 'Unable to connect to the backend server',
      actionLabel: 'Reconnect now',
    });
  });

  it('returns the workloads disconnected banner copy', () => {
    expect(getWorkloadsDisconnectedBannerState(true)).toEqual({
      title: 'Connection lost',
      description: 'Real-time data is reconnecting. Showing last-known state.',
      actionLabel: 'Reconnect',
    });
    expect(getWorkloadsDisconnectedBannerState(false)).toEqual({
      title: 'Connection lost',
      description: 'Real-time data is currently unavailable. Showing last-known state.',
      actionLabel: 'Reconnect',
    });
  });

  it('returns the workloads unavailable and empty states', () => {
    expect(getWorkloadsUnavailableState()).toEqual({
      title: 'Workloads unavailable',
      description: 'Real-time workload data is currently unavailable. Reconnect to try again.',
      actionLabel: 'Reconnect',
    });
    expect(getWorkloadsNoInventoryState()).toEqual({
      title: 'No workload inventory available',
      description:
        'Pulse has infrastructure sources, but no VM, container, or pod inventory is available right now. Review source credentials, permissions, and collection status in Settings → Infrastructure.',
      actionLabel: 'Review infrastructure sources',
    });
    expect(getWorkloadsNoResourcesState()).toEqual({
      title: 'Connect your first infrastructure source',
      description:
        'Workloads appear after Pulse receives its first monitored system. Add an infrastructure source with API inventory, Agent telemetry, or both, then return here to inspect running VMs, containers, and pods.',
      actionLabel: 'Add infrastructure source',
    });
  });
});
