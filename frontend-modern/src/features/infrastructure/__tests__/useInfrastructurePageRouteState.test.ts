import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { ROUTE_STATE_REPLACE_OPTIONS } from '@/utils/routeStateNavigation';

import { useInfrastructurePageRouteState } from '../useInfrastructurePageRouteState';

let locationPath = '/infrastructure';
let locationSearch = '';
const navigateSpy = vi.fn();

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({
    get pathname() {
      return locationPath;
    },
    get search() {
      return locationSearch;
    },
  }),
  useNavigate: () => navigateSpy,
}));

const makeResource = (id: string): Resource =>
  ({
    id,
    name: id,
    displayName: id,
    sourceType: 'agent',
    status: 'online',
    type: 'agent',
  }) as Resource;

describe('useInfrastructurePageRouteState', () => {
  beforeEach(() => {
    locationPath = '/infrastructure';
    locationSearch = '';
    navigateSpy.mockReset();
    vi.useFakeTimers();
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
      callback(0);
      return 1;
    });
    vi.stubGlobal('cancelAnimationFrame', vi.fn());
    Object.defineProperty(window.history, 'scrollRestoration', {
      configurable: true,
      value: 'auto',
      writable: true,
    });
    window.scrollTo = vi.fn() as typeof window.scrollTo;
  });

  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  const setup = () => {
    const [resources] = createSignal<Resource[]>([makeResource('agent-1'), makeResource('agent-2')]);
    const [selectedSource, setSelectedSource] = createSignal('');
    const [searchQuery, setSearchQuery] = createSignal('');
    const [initialLoadComplete] = createSignal(true);

    const rendered = renderHook(() =>
      useInfrastructurePageRouteState({
        resources,
        filteredResources: resources,
        initialLoadComplete,
        selectedSource,
        setSelectedSource,
        searchQuery,
        setSearchQuery,
      }),
    );

    return { ...rendered, setSelectedSource, setSearchQuery };
  };

  it('hydrates an inbound resource deep link into drawer state', () => {
    locationSearch = '?resource=agent-1';

    const { result } = setup();

    expect(result.expandedResourceId()).toBe('agent-1');
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  it('keeps local drawer expansion off the route navigation path', () => {
    const { result } = setup();

    result.setExpandedResourceId('agent-2');
    vi.runAllTimers();

    expect(result.expandedResourceId()).toBe('agent-2');
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  it('keeps infrastructure filters route-backed', () => {
    const { setSelectedSource } = setup();

    setSelectedSource('docker');
    vi.runAllTimers();

    expect(navigateSpy).toHaveBeenCalledWith(
      '/infrastructure?source=docker',
      ROUTE_STATE_REPLACE_OPTIONS,
    );
  });
});
