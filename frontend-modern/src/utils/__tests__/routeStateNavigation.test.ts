import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  ROUTE_STATE_REPLACE_OPTIONS,
  createRouteStateNavigateScheduler,
  markRouteStateDeliberateScroll,
} from '@/utils/routeStateNavigation';
import {
  clearPendingAppShellRestoreTop,
  readPendingAppShellRestoreTop,
} from '@/utils/appShellScrollRestoration';

describe('routeStateNavigation', () => {
  const scrollToSpy = vi.fn();
  let currentScrollX = 24;
  let currentScrollY = 320;

  beforeEach(() => {
    vi.useFakeTimers();
    clearPendingAppShellRestoreTop();
    scrollToSpy.mockReset();
    currentScrollX = 24;
    currentScrollY = 320;
    Object.defineProperty(window, 'scrollX', {
      configurable: true,
      get: () => currentScrollX,
    });
    Object.defineProperty(window, 'scrollY', {
      configurable: true,
      get: () => currentScrollY,
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
    scrollToSpy.mockImplementation((x: number, y: number) => {
      currentScrollX = x;
      currentScrollY = y;
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

  it('captures the app scroll shell before same-path route state navigation', () => {
    const navigate = vi.fn();
    let currentPath = '/infrastructure?source=proxmox-pve';
    const scheduler = createRouteStateNavigateScheduler(navigate, () => currentPath);
    const shell = document.createElement('div');
    shell.className = 'app-scroll-shell';
    shell.scrollTop = 55;
    document.body.appendChild(shell);

    scheduler.schedule('/infrastructure?source=proxmox-pve&resource=agent-123');
    vi.runAllTimers();

    expect(navigate).toHaveBeenCalledWith(
      '/infrastructure?source=proxmox-pve&resource=agent-123',
      ROUTE_STATE_REPLACE_OPTIONS,
    );
    expect(readPendingAppShellRestoreTop()).toBe(55);
  });

  it('skips redundant navigations to the current path', () => {
    const navigate = vi.fn();
    const scheduler = createRouteStateNavigateScheduler(navigate, () => '/recovery?rollupId=abc');

    scheduler.schedule('/recovery?rollupId=abc');
    vi.runAllTimers();

    expect(navigate).not.toHaveBeenCalled();
    expect(scrollToSpy).not.toHaveBeenCalled();
  });

  it('stops replaying preserved scroll after a later deliberate scroll change', () => {
    const navigate = vi.fn();
    let currentPath = '/infrastructure?source=proxmox-pve';
    const scheduler = createRouteStateNavigateScheduler(navigate, () => currentPath);

    scheduler.schedule('/infrastructure?source=proxmox-pve&resource=agent-123');
    vi.advanceTimersByTime(0);

    expect(scrollToSpy).toHaveBeenCalledTimes(1);

    currentScrollY = 540;
    vi.runAllTimers();

    expect(scrollToSpy).toHaveBeenCalledTimes(1);
  });

  it('suppresses preserved scroll replays while a deliberate scroll handoff is active', () => {
    const navigate = vi.fn();
    let currentPath = '/workloads?type=app-container';
    const scheduler = createRouteStateNavigateScheduler(navigate, () => currentPath);

    markRouteStateDeliberateScroll();
    scheduler.schedule('/workloads?type=app-container&resource=guest-1');
    vi.runAllTimers();

    expect(navigate).toHaveBeenCalledWith(
      '/workloads?type=app-container&resource=guest-1',
      ROUTE_STATE_REPLACE_OPTIONS,
    );
    expect(scrollToSpy).not.toHaveBeenCalled();
  });
});
