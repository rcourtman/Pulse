import { createSignal } from 'solid-js';
import { renderHook, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { apiFetch } from '@/utils/apiClient';
import { showSuccess } from '@/utils/toast';
import { useAuditLogPanelState, type AuditEvent } from '../useAuditLogPanelState';

// Reactive location search. The mock factory closes over the signal readers
// lazily (same pattern the sibling spec uses for `navigate`), so the filter
// createEffects inside the hook genuinely re-run when a setter mutates the URL.
const [readLocationSearch, writeLocationSearch] = createSignal('');

const panelState = vi.hoisted(() => ({
  license: {
    hasFeature: (feature: string): boolean => feature === 'audit_logging',
    runtimeLoaded: true,
    capabilityBlock: undefined as { reason?: string; action_url?: string } | undefined,
  },
  hideUpgradePrompts: false,
}));

const navigate = vi.hoisted(() => vi.fn());

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({
    pathname: '/settings/security',
    get search() {
      return readLocationSearch();
    },
  }),
  useNavigate: () => navigate,
}));

vi.mock('@/stores/license', () => ({
  getRuntimeCapabilityBlock: () => panelState.license.capabilityBlock,
  hasFeature: (feature: string) => panelState.license.hasFeature(feature),
  loadRuntimeCapabilities: vi.fn().mockResolvedValue(undefined),
  runtimeCapabilitiesLoaded: () => panelState.license.runtimeLoaded,
}));

vi.mock('@/stores/licenseCommercial', () => ({
  getUpgradeActionDestination: () => ({ href: '/upgrade', external: false }),
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesUpgradePrompts: () => panelState.hideUpgradePrompts,
}));

vi.mock('@/utils/upgradeNavigation', () => ({
  resolveUpgradeDestination: (url?: string) => ({
    href: url ?? '/fallback',
    external: false,
    hardNavigation: false,
    newTab: false,
    preserveOpener: false,
  }),
}));

vi.mock('@/utils/toast', () => ({
  showSuccess: vi.fn(),
  showToast: vi.fn(),
  showWarning: vi.fn(),
}));

vi.mock('@/utils/apiClient', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/utils/apiClient')>();
  return {
    ...actual,
    apiFetch: vi.fn(),
    apiErrorFromResponse: vi.fn(),
  };
});

type Deferred<T> = {
  promise: Promise<T>;
  resolve: (value: T) => void;
};

const deferred = <T,>(): Deferred<T> => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => {
    resolve = next;
  });
  return { promise, resolve };
};

const JSON_HEADERS = { 'Content-Type': 'application/json' };

const signedEvent = (id: string): AuditEvent => ({
  id,
  timestamp: '2026-07-24T09:00:00Z',
  event: 'login',
  user: 'operator',
  ip: '127.0.0.1',
  path: '/api/auth',
  success: true,
  details: id,
  signature: `sig-${id}`,
});

const unsignedEvent = (id: string): AuditEvent => ({
  id,
  timestamp: '2026-07-24T09:00:00Z',
  event: 'login',
  user: 'operator',
  ip: '127.0.0.1',
  path: '/api/auth',
  success: true,
  details: id,
});

const auditListResponse = (
  events: AuditEvent[],
  total: number = events.length,
  persistentLogging = false,
): Response =>
  new Response(JSON.stringify({ events, total, persistentLogging }), {
    status: 200,
    headers: JSON_HEADERS,
  });

const verifyResponse = (
  body: { available: boolean; verified?: boolean; message?: string },
  status = 200,
  statusText = '',
): Response => new Response(JSON.stringify(body), { status, statusText, headers: JSON_HEADERS });

const isVerifyCall = (url: unknown): boolean => typeof url === 'string' && url.includes('/verify');

const renderIdle = async (options: { featureEnabled?: boolean; locationSearch?: string } = {}) => {
  if (options.locationSearch !== undefined) writeLocationSearch(options.locationSearch);
  if (options.featureEnabled === false) panelState.license.hasFeature = () => false;
  const hook = renderHook(() => useAuditLogPanelState());
  await waitFor(() => expect(hook.result.loading()).toBe(false));
  return hook;
};

describe('useAuditLogPanelState branch coverage', () => {
  beforeEach(() => {
    window.localStorage.clear();
    writeLocationSearch('');
    panelState.license.hasFeature = (feature: string): boolean => feature === 'audit_logging';
    panelState.license.runtimeLoaded = true;
    panelState.license.capabilityBlock = undefined;
    panelState.hideUpgradePrompts = false;
    navigate.mockReset();
    navigate.mockImplementation((url: string) => {
      const q = String(url).indexOf('?');
      writeLocationSearch(q >= 0 ? String(url).slice(q + 1) : '');
    });
    vi.mocked(apiFetch).mockReset();
    vi.mocked(showSuccess).mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('URL filter setters + updateSearchParam', () => {
    it('setEventFilter writes an allowed event value into the search string', async () => {
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      result.setEventFilter('login');
      expect(result.eventFilter()).toBe('login');
      expect(navigate).toHaveBeenLastCalledWith('/settings/security?event=login', {
        replace: true,
      });
      cleanup();
    });

    it('setEventFilter drops the event param when the value normalizes to empty', async () => {
      writeLocationSearch('event=login');
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      result.setEventFilter('');
      expect(result.eventFilter()).toBe('');
      // updateSearchParam must omit the '?' entirely once no params remain.
      expect(navigate).toHaveBeenLastCalledWith('/settings/security', { replace: true });
      cleanup();
    });

    it('setUserFilter writes a non-empty user value into the search string', async () => {
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      result.setUserFilter('alice');
      expect(result.userFilter()).toBe('alice');
      expect(readLocationSearch()).toBe('user=alice');
      cleanup();
    });

    it('setUserFilter drops the user param when the value is empty', async () => {
      writeLocationSearch('user=alice');
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      result.setUserFilter('');
      expect(result.userFilter()).toBe('');
      expect(readLocationSearch()).toBe('');
      cleanup();
    });

    it('setSuccessFilter writes a non-default success value into the search string', async () => {
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      result.setSuccessFilter('failed');
      expect(result.successFilter()).toBe('failed');
      expect(readLocationSearch()).toBe('success=failed');
      cleanup();
    });

    it('setSuccessFilter drops the success param when the value resolves to the default', async () => {
      writeLocationSearch('success=failed');
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      result.setSuccessFilter('all');
      expect(result.successFilter()).toBe('all');
      expect(readLocationSearch()).toBe('');
      cleanup();
    });

    it('setVerificationFilter writes a non-default verification value into the search string', async () => {
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      result.setVerificationFilter('needs');
      expect(result.verificationFilter()).toBe('needs');
      expect(readLocationSearch()).toBe('verification=needs');
      cleanup();
    });

    it('setVerificationFilter drops the verification param when the value resolves to the default', async () => {
      writeLocationSearch('verification=needs');
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      result.setVerificationFilter('all');
      expect(result.verificationFilter()).toBe('all');
      expect(readLocationSearch()).toBe('');
      cleanup();
    });
  });

  describe('activeFilterCount and activeFilterChips', () => {
    it('reports zero filters and no chips when the URL is clean', async () => {
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      expect(result.activeFilterCount()).toBe(0);
      expect(result.activeFilterChips()).toEqual([]);
      cleanup();
    });

    it('counts all four filters and emits one chip per active filter', async () => {
      writeLocationSearch('event=login&user=alice&success=failed&verification=needs');
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      expect(result.activeFilterCount()).toBe(4);
      expect(result.activeFilterChips().map((chip) => chip.label)).toEqual([
        'Event: login',
        'User: alice',
        'Success: failed',
        'Verification: needs',
      ]);
      cleanup();
    });
  });

  describe('clearFilters', () => {
    it('clears every filter and toasts when filters were active', async () => {
      writeLocationSearch('event=login&user=alice');
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      expect(result.activeFilterCount()).toBe(2);
      result.clearFilters();
      expect(showSuccess).toHaveBeenCalledWith('Audit filters cleared');
      expect(readLocationSearch()).toBe('');
      expect(result.activeFilterCount()).toBe(0);
      cleanup();
    });

    it('clears quietly without toasting when no filters were active', async () => {
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      result.clearFilters();
      expect(showSuccess).not.toHaveBeenCalled();
      expect(readLocationSearch()).toBe('');
      cleanup();
    });
  });

  describe('clearFilterChip', () => {
    it('clears the event filter when keyed on "event"', async () => {
      writeLocationSearch('event=login');
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      expect(result.eventFilter()).toBe('login');
      result.clearFilterChip('event');
      expect(result.eventFilter()).toBe('');
      cleanup();
    });

    it('clears the user filter when keyed on "user"', async () => {
      writeLocationSearch('user=alice');
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      expect(result.userFilter()).toBe('alice');
      result.clearFilterChip('user');
      expect(result.userFilter()).toBe('');
      cleanup();
    });

    it('clears the success filter when keyed on "success"', async () => {
      writeLocationSearch('success=failed');
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      expect(result.successFilter()).toBe('failed');
      result.clearFilterChip('success');
      expect(result.successFilter()).toBe('all');
      cleanup();
    });

    it('clears the verification filter when keyed on "verification"', async () => {
      writeLocationSearch('verification=needs');
      const { result, cleanup } = await renderIdle({ featureEnabled: false });
      expect(result.verificationFilter()).toBe('needs');
      result.clearFilterChip('verification');
      expect(result.verificationFilter()).toBe('all');
      cleanup();
    });
  });

  describe('upgrade prompt and feature-gate accessors', () => {
    it('exposes upgrade prompts and the "View plans" label when audit logging is simply unlicensed', async () => {
      panelState.license.hasFeature = () => false;
      const { result, cleanup } = await renderIdle();
      expect(result.paidRuntimeRequired()).toBe(false);
      expect(result.showUpgradePrompts()).toBe(true);
      expect(result.showFeatureGateAction()).toBe(true);
      expect(result.upgradeActionLabel()).toBe('View plans');
      cleanup();
    });

    it('hides upgrade prompts and switches to the Pro download label when a paid runtime is required', async () => {
      panelState.license.hasFeature = () => false;
      panelState.license.capabilityBlock = {
        reason: 'paid_runtime_required',
        action_url: '/download',
      };
      const { result, cleanup } = await renderIdle();
      expect(result.paidRuntimeRequired()).toBe(true);
      expect(result.showUpgradePrompts()).toBe(false);
      expect(result.showFeatureGateAction()).toBe(true);
      expect(result.upgradeActionLabel()).toBe('Download Pulse Pro');
      cleanup();
    });

    it('hides the feature-gate action entirely when the presentation policy suppresses upgrade prompts', async () => {
      panelState.license.hasFeature = () => false;
      panelState.hideUpgradePrompts = true;
      const { result, cleanup } = await renderIdle();
      expect(result.showUpgradePrompts()).toBe(false);
      expect(result.showFeatureGateAction()).toBe(false);
      expect(result.upgradeActionLabel()).toBe('View plans');
      cleanup();
    });
  });

  describe('verifyEvent', () => {
    it('is a no-op when audit logging is disabled', async () => {
      panelState.license.hasFeature = () => false;
      vi.mocked(apiFetch).mockImplementation(async () => auditListResponse([]));
      const { result, cleanup } = await renderIdle();
      await result.verifyEvent(signedEvent('a'));
      expect(apiFetch).not.toHaveBeenCalled();
      expect(result.verifying()).toEqual({});
      expect(result.verification()['a']).toBeUndefined();
      cleanup();
    });

    it('is a no-op when the event carries no signature', async () => {
      vi.mocked(apiFetch).mockImplementation(async (url: string) => {
        if (isVerifyCall(url)) return verifyResponse({ available: true, verified: true });
        return auditListResponse([]);
      });
      const { result, cleanup } = await renderIdle();
      const callsBefore = vi.mocked(apiFetch).mock.calls.length;
      await result.verifyEvent(unsignedEvent('a'));
      expect(vi.mocked(apiFetch).mock.calls.length).toBe(callsBefore);
      expect(result.verification()['a']).toBeUndefined();
      cleanup();
    });

    it('records a "verified" status when the signature checks out', async () => {
      vi.mocked(apiFetch).mockImplementation(async (url: string) => {
        if (isVerifyCall(url)) {
          return verifyResponse({ available: true, verified: true, message: 'matched' });
        }
        return auditListResponse([]);
      });
      const { result, cleanup } = await renderIdle();
      const event = signedEvent('a');
      await result.verifyEvent(event);
      expect(result.verification()[event.id]).toEqual({
        status: 'compatibility',
        message: 'matched',
      });
      expect(result.verifying()[event.id]).toBe(false);
      cleanup();
    });

    it('records an "unavailable" status when verification is not available for the event', async () => {
      vi.mocked(apiFetch).mockImplementation(async (url: string) => {
        if (isVerifyCall(url)) return verifyResponse({ available: false, message: 'no key' });
        return auditListResponse([]);
      });
      const { result, cleanup } = await renderIdle();
      const event = signedEvent('a');
      await result.verifyEvent(event);
      expect(result.verification()[event.id]).toEqual({
        status: 'unavailable',
        message: 'no key',
      });
      cleanup();
    });

    it('records a "failed" status when verification ran but the signature did not match', async () => {
      vi.mocked(apiFetch).mockImplementation(async (url: string) => {
        if (isVerifyCall(url)) {
          return verifyResponse({ available: true, verified: false, message: 'mismatch' });
        }
        return auditListResponse([]);
      });
      const { result, cleanup } = await renderIdle();
      const event = signedEvent('a');
      await result.verifyEvent(event);
      expect(result.verification()[event.id]).toEqual({ status: 'invalid', message: 'mismatch' });
      cleanup();
    });

    it('records an "error" status when the verify endpoint returns a non-OK response', async () => {
      vi.mocked(apiFetch).mockImplementation(async (url: string) => {
        if (isVerifyCall(url)) {
          return verifyResponse({ available: true }, 500, 'Internal Server Error');
        }
        return auditListResponse([]);
      });
      const { result, cleanup } = await renderIdle();
      const event = signedEvent('a');
      await result.verifyEvent(event);
      expect(result.verification()[event.id]?.status).toBe('error');
      expect(result.verification()[event.id]?.message).toContain('Failed to verify signature');
      cleanup();
    });
  });

  describe('signed-event inspection (hasSignedEvents / hasResumeEvents / resumeCount)', () => {
    it('reports no signed events when every event lacks a signature', async () => {
      vi.mocked(apiFetch).mockResolvedValue(
        auditListResponse([unsignedEvent('a'), unsignedEvent('b')]),
      );
      const { result, cleanup } = await renderIdle();
      expect(result.hasSignedEvents()).toBe(false);
      expect(result.hasResumeEvents()).toBe(false);
      expect(result.resumeCount()).toBe(0);
      cleanup();
    });

    it('counts every unchecked signed event as resumable', async () => {
      vi.mocked(apiFetch).mockResolvedValue(
        auditListResponse([signedEvent('a'), unsignedEvent('b'), signedEvent('c')]),
      );
      const { result, cleanup } = await renderIdle();
      expect(result.hasSignedEvents()).toBe(true);
      expect(result.hasResumeEvents()).toBe(true);
      expect(result.resumeCount()).toBe(2);
      cleanup();
    });

    it('drops an event from the resume set once it verifies successfully', async () => {
      vi.mocked(apiFetch).mockImplementation(async (url: string) => {
        if (isVerifyCall(url)) return verifyResponse({ available: true, verified: true });
        return auditListResponse([signedEvent('a'), signedEvent('b')]);
      });
      const { result, cleanup } = await renderIdle();
      expect(result.resumeCount()).toBe(2);
      await result.verifyEvent(result.events()[0]);
      expect(result.resumeCount()).toBe(1);
      await result.verifyEvent(result.events()[1]);
      expect(result.resumeCount()).toBe(0);
      expect(result.hasResumeEvents()).toBe(false);
      cleanup();
    });

    it('keeps failed events resumable while treating unavailable ones as settled', async () => {
      let verifyCount = 0;
      vi.mocked(apiFetch).mockImplementation(async (url: string) => {
        if (isVerifyCall(url)) {
          verifyCount += 1;
          return verifyCount === 1
            ? verifyResponse({ available: true, verified: false })
            : verifyResponse({ available: false });
        }
        return auditListResponse([signedEvent('a'), signedEvent('b')]);
      });
      const { result, cleanup } = await renderIdle();
      expect(result.resumeCount()).toBe(2);
      await result.verifyEvent(result.events()[0]);
      expect(result.resumeCount()).toBe(2);
      await result.verifyEvent(result.events()[1]);
      expect(result.resumeCount()).toBe(1);
      cleanup();
    });
  });

  it('uses authoritative server assurance for totals, filters, and resume decisions', async () => {
    const events: AuditEvent[] = [
      {
        ...signedEvent('strong'),
        signature_version: 'v3',
        signature_status: 'strong',
        signature_assurance: 'strong',
      },
      {
        ...signedEvent('compatibility'),
        signature_version: 'legacy',
        signature_status: 'compatibility',
        signature_assurance: 'compatibility',
      },
      {
        ...signedEvent('invalid'),
        signature_version: 'v3',
        signature_status: 'invalid',
        signature_assurance: 'none',
      },
      {
        ...signedEvent('unknown'),
        signature_version: 'unknown',
        signature_status: 'unknown',
        signature_assurance: 'none',
      },
      {
        ...unsignedEvent('unsigned'),
        signature_version: 'unsigned',
        signature_status: 'unsigned',
        signature_assurance: 'none',
      },
    ];
    vi.mocked(apiFetch).mockResolvedValue(auditListResponse(events));

    const { result, cleanup } = await renderIdle({
      locationSearch: '?verification=compatibility',
    });
    expect(result.verificationSummary()).toMatchObject({
      total: 5,
      signed: 4,
      strong: 1,
      compatibility: 1,
      invalid: 1,
      unknown: 1,
      unsigned: 1,
      unchecked: 0,
    });
    expect(result.filteredEvents().map((event) => event.id)).toEqual(['compatibility']);
    expect(result.resumeCount()).toBe(1);
    expect(result.verification()['compatibility']).toEqual({
      status: 'compatibility',
      message: 'Historical signature verified with compatibility assurance only',
    });
    cleanup();
  });

  it('does not re-verify events whose list projection already has an authoritative status', async () => {
    const event: AuditEvent = {
      ...signedEvent('projected'),
      signature_version: 'v3',
      signature_status: 'strong',
      signature_assurance: 'strong',
    };
    vi.mocked(apiFetch).mockImplementation(async (url: string) => {
      if (isVerifyCall(url)) {
        throw new Error('projected event was redundantly verified');
      }
      return auditListResponse([event], 1, true);
    });

    const { result, cleanup } = await renderIdle();
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(result.verification()['projected']?.status).toBe('strong');
    expect(vi.mocked(apiFetch).mock.calls.filter(([url]) => isVerifyCall(url))).toHaveLength(0);
    cleanup();
  });

  describe('verifyAllLabel', () => {
    it('reads "Verify All" while no batch verification is running', async () => {
      vi.mocked(apiFetch).mockResolvedValue(auditListResponse([signedEvent('a')]));
      const { result, cleanup } = await renderIdle();
      expect(result.verifyAllLabel()).toBe('Verify All');
      cleanup();
    });

    it('reports live progress while a batch verification is paused mid-flight', async () => {
      const firstVerify = deferred<Response>();
      let verifyCount = 0;
      vi.mocked(apiFetch).mockImplementation(async (url: string) => {
        if (isVerifyCall(url)) {
          verifyCount += 1;
          if (verifyCount === 1) return firstVerify.promise;
          return verifyResponse({ available: true, verified: true });
        }
        return auditListResponse([signedEvent('a'), signedEvent('b')]);
      });
      const { result, cleanup } = await renderIdle();
      result.verifyAll();
      expect(result.verifyAllLabel()).toBe('Verifying 0 of 2');
      firstVerify.resolve(verifyResponse({ available: true, verified: true }));
      await waitFor(() => expect(result.verifyingAll()).toBe(false));
      expect(result.verifyAllLabel()).toBe('Verify All');
      cleanup();
    });

    it('falls back to the ellipsis label when a fetch resets the verify-all total mid-flight', async () => {
      const firstVerify = deferred<Response>();
      let verifyCount = 0;
      vi.mocked(apiFetch).mockImplementation(async (url: string) => {
        if (isVerifyCall(url)) {
          verifyCount += 1;
          if (verifyCount === 1) return firstVerify.promise;
          return verifyResponse({ available: true, verified: true });
        }
        return auditListResponse([signedEvent('a'), signedEvent('b')]);
      });
      const { result, cleanup } = await renderIdle();
      result.verifyAll();
      expect(result.verifyAllLabel()).toBe('Verifying 0 of 2');
      result.refresh();
      expect(result.verifyAllLabel()).toBe('Verifying…');
      firstVerify.resolve(verifyResponse({ available: true, verified: true }));
      await waitFor(() => expect(result.verifyingAll()).toBe(false));
      cleanup();
    });
  });

  describe('paging accessors (hasNextPage / totalPages)', () => {
    it('clamps to a single page with no next page when there are no events', async () => {
      vi.mocked(apiFetch).mockResolvedValue(auditListResponse([]));
      const { result, cleanup } = await renderIdle();
      expect(result.totalPages()).toBe(1);
      expect(result.pageNumber()).toBe(1);
      expect(result.hasNextPage()).toBe(false);
      cleanup();
    });

    it('exposes a next page while on the first page of a multi-page set', async () => {
      vi.mocked(apiFetch).mockResolvedValue(
        auditListResponse(
          Array.from({ length: 100 }, (_, index) => unsignedEvent(`e${index}`)),
          250,
        ),
      );
      const { result, cleanup } = await renderIdle();
      expect(result.totalPages()).toBe(3);
      expect(result.pageNumber()).toBe(1);
      expect(result.hasNextPage()).toBe(true);
      cleanup();
    });

    it('reports no next page while on the final page', async () => {
      window.localStorage.setItem('pulse-audit-page-offset', '200');
      vi.mocked(apiFetch).mockResolvedValue(
        auditListResponse(
          Array.from({ length: 50 }, (_, index) => unsignedEvent(`e${index}`)),
          250,
        ),
      );
      const { result, cleanup } = await renderIdle();
      expect(result.pageNumber()).toBe(3);
      expect(result.totalPages()).toBe(3);
      expect(result.hasNextPage()).toBe(false);
      cleanup();
    });
  });

  describe('resetPaging (exercised via resetPreferences)', () => {
    it('zeroes the stored offset and clears the page input', async () => {
      window.localStorage.setItem('pulse-audit-page-offset', '200');
      vi.mocked(apiFetch).mockResolvedValue(auditListResponse([], 250));
      const { result, cleanup } = await renderIdle();
      expect(result.pageOffset()).toBe(200);
      result.setPageInput('3');
      expect(result.pageInput()).toBe('3');
      result.resetPreferences();
      expect(result.pageOffset()).toBe(0);
      expect(result.pageInput()).toBe('');
      expect(showSuccess).toHaveBeenCalledWith('Audit preferences reset');
      cleanup();
    });
  });

  describe('refetchFromFirstPage (exercised via the server-side filter effects)', () => {
    it('re-requests the audit list from offset zero when an event filter is applied', async () => {
      window.localStorage.setItem('pulse-audit-page-offset', '200');
      vi.mocked(apiFetch).mockResolvedValue(auditListResponse([], 250));
      const { result, cleanup } = await renderIdle();
      expect(result.pageOffset()).toBe(200);
      result.setEventFilter('login');
      await waitFor(() => {
        const lastURL = vi.mocked(apiFetch).mock.calls.at(-1)?.[0];
        expect(String(lastURL)).toContain('offset=0');
      });
      expect(result.eventFilter()).toBe('login');
      expect(result.pageOffset()).toBe(0);
      cleanup();
    });
  });
});
