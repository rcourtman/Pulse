import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  ROUTE_STATE_REPLACE_OPTIONS,
  createRouteStateNavigateScheduler,
} from '@/utils/routeStateNavigation';

describe('routeStateNavigation', () => {
  const scrollToSpy = vi.fn();

  beforeEach(() => {
    vi.useFakeTimers();
    scrollToSpy.mockReset();
    Object.defineProperty(window, 'scrollX', {
      configurable: true,
      value: 24,
    });
    Object.defineProperty(window, 'scrollY', {
      configurable: true,
      value: 320,
    });
    Object.defineProperty(window.navigator, 'userAgent', {
      configurable: true,
      value: 'Mozilla/5.0',
    });
    Object.defineProperty(window.history, 'scrollRestoration', {
      configurable: true,
      value: 'auto',
      writable: true,
    });
    window.scrollTo = scrollToSpy as typeof window.scrollTo;
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('preserves scroll when replacing query state on the same pathname', () => {
    const navigate = vi.fn();
    let currentPath = '/infrastructure?source=proxmox-pve';
    const scheduler = createRouteStateNavigateScheduler(navigate, () => currentPath);

    scheduler.schedule('/infrastructure?source=proxmox-pve&resource=agent-123');
    vi.runAllTimers();

    expect(navigate).toHaveBeenCalledWith(
      '/infrastructure?source=proxmox-pve&resource=agent-123',
      ROUTE_STATE_REPLACE_OPTIONS,
    );
    expect(scrollToSpy).toHaveBeenCalledWith(24, 320);
  });

  it('skips redundant navigations to the current path', () => {
    const navigate = vi.fn();
    const scheduler = createRouteStateNavigateScheduler(navigate, () => '/recovery?rollupId=abc');

    scheduler.schedule('/recovery?rollupId=abc');
    vi.runAllTimers();

    expect(navigate).not.toHaveBeenCalled();
    expect(scrollToSpy).not.toHaveBeenCalled();
  });
});
