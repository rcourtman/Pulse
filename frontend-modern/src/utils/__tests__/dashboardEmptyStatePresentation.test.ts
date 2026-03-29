import { describe, expect, it } from 'vitest';
import {
  getDashboardDisconnectedBannerState,
  getDashboardDisconnectedState,
  getDashboardGuestsEmptyState,
  getDashboardInfrastructureEmptyState,
  getDashboardLoadingState,
  getDashboardNoResourcesState,
  getDashboardUnavailableState,
} from '@/utils/dashboardEmptyStatePresentation';

describe('dashboardEmptyStatePresentation', () => {
  it('returns the infrastructure onboarding empty state', () => {
    expect(getDashboardInfrastructureEmptyState()).toEqual({
      title: 'No infrastructure hosts connected',
      description:
        'To start using Pulse, first add your infrastructure in Settings → Infrastructure → Install on a host. If you want an API-backed platform such as Proxmox or TrueNAS instead, use Settings → Infrastructure → Platform connections.',
      actionLabel: 'Open infrastructure setup',
    });
  });

  it('returns the guest filter empty state for empty search', () => {
    expect(getDashboardGuestsEmptyState('')).toEqual({
      title: 'No guests found',
      description: 'No guests match your current filters',
    });
  });

  it('returns the guest filter empty state for a search term', () => {
    expect(getDashboardGuestsEmptyState('  proxmox  ')).toEqual({
      title: 'No guests found',
      description: 'No guests match your search "proxmox"',
    });
  });

  it('returns the dashboard loading-state copy', () => {
    expect(getDashboardLoadingState(false)).toEqual({
      title: 'Loading dashboard data...',
      description: 'Connecting to monitoring service',
    });
    expect(getDashboardLoadingState(true)).toEqual({
      title: 'Loading dashboard data...',
      description: 'Reconnecting to monitoring service…',
    });
  });

  it('returns the dashboard disconnected-state copy', () => {
    expect(getDashboardDisconnectedState(true)).toEqual({
      title: 'Connection lost',
      description: 'Attempting to reconnect…',
      actionLabel: undefined,
    });
    expect(getDashboardDisconnectedState(false)).toEqual({
      title: 'Connection lost',
      description: 'Unable to connect to the backend server',
      actionLabel: 'Reconnect now',
    });
  });

  it('returns the dashboard disconnected banner copy', () => {
    expect(getDashboardDisconnectedBannerState(true)).toEqual({
      title: 'Connection lost',
      description: 'Real-time data is reconnecting. Showing last-known state.',
      actionLabel: 'Reconnect',
    });
    expect(getDashboardDisconnectedBannerState(false)).toEqual({
      title: 'Connection lost',
      description: 'Real-time data is currently unavailable. Showing last-known state.',
      actionLabel: 'Reconnect',
    });
  });

  it('returns the dashboard unavailable and empty states', () => {
    expect(getDashboardUnavailableState()).toEqual({
      title: 'Dashboard unavailable',
      description: 'Real-time dashboard data is currently unavailable. Reconnect to try again.',
      actionLabel: 'Reconnect',
    });
    expect(getDashboardNoResourcesState()).toEqual({
      title: 'No resources yet',
      description:
        'Start by opening Settings → Infrastructure → Install on a host and connecting the first system you want Pulse to monitor. Your dashboard overview will appear here once that system starts reporting.',
      actionLabel: 'Open infrastructure install',
    });
  });
});
