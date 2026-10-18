import { cleanup, fireEvent, render, renderHook, screen, waitFor } from '@solidjs/testing-library';
import { Show, createSignal, onMount } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { apiFetchJSON } from '@/utils/apiClient';
import { AlertEmailDestinationsSection } from '../AlertEmailDestinationsSection';
import { useAlertDestinationsState } from '../useAlertDestinationsState';

vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: vi.fn() }));
vi.mock('@/stores/license', () => ({ hasFeature: () => false }));
vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    getDeadManConfig: vi.fn().mockResolvedValue({ pingUrl: '', configured: false }),
    updateDeadManConfig: vi.fn().mockResolvedValue({ success: true, configured: false }),
  },
}));

const savedConfig = {
  enabled: true,
  provider: '',
  server: 'smtp.example.test',
  port: 587,
  username: 'ops@example.test',
  password: '***REDACTED***',
  from: 'pulse@example.test',
  to: ['alerts@example.test'],
  tls: false,
  startTLS: true,
  rateLimit: 17,
  tagFilter: ['production'],
  tagFilterMode: 'any',
  minimumSeverity: 'warning',
};

function writtenEmailConfig() {
  const calls = vi
    .mocked(apiFetchJSON)
    .mock.calls.filter(
      ([path, options]) => path === '/api/notifications/email' && options?.method === 'PUT',
    );
  expect(calls).toHaveLength(1);
  return JSON.parse(String(calls[0][1]?.body));
}

describe('email rate-limit persistence', () => {
  let loadedRateLimit: number | undefined;
  let loadedEnabled: boolean;

  beforeEach(() => {
    loadedRateLimit = 17;
    loadedEnabled = true;
    vi.mocked(apiFetchJSON).mockReset();
    vi.mocked(apiFetchJSON).mockImplementation(
      async <T,>(path: string, options?: Parameters<typeof apiFetchJSON>[1]): Promise<T> => {
        if (path === '/api/notifications/email') {
          return (
            options?.method === 'PUT'
              ? { success: true }
              : { ...savedConfig, rateLimit: loadedRateLimit, enabled: loadedEnabled }
          ) as T;
        }
        if (path === '/api/notifications/apprise') return { enabled: false, targets: [] } as T;
        if (path === '/api/notifications/webhooks' || path === '/api/notifications/email-providers')
          return [] as T;
        throw new Error('Unexpected synthetic API call');
      },
    );
  });

  afterEach(cleanup);

  it('keeps the loaded rate limit when saving an unrelated email edit', async () => {
    const { result } = renderHook(() =>
      useAlertDestinationsState({ activeTab: () => 'destinations' }),
    );
    await result.loadDestinations();
    result.setEmailConfig({ ...result.emailConfig(), from: 'new@example.test' });
    await result.saveDestinations();

    expect(writtenEmailConfig()).toEqual({ ...savedConfig, from: 'new@example.test' });
  });

  it.each([0, 1, 120])(
    'preserves an explicit saved rate limit of %i on the wire',
    async (limit) => {
      loadedRateLimit = limit;
      const { result } = renderHook(() =>
        useAlertDestinationsState({ activeTab: () => 'destinations' }),
      );
      await result.loadDestinations();
      await result.saveDestinations();

      expect(writtenEmailConfig().rateLimit).toBe(limit);
    },
  );

  it('sends the existing default when the saved config has no rate limit', async () => {
    loadedRateLimit = undefined;
    const { result } = renderHook(() =>
      useAlertDestinationsState({ activeTab: () => 'destinations' }),
    );
    await result.loadDestinations();
    await result.saveDestinations();

    expect(writtenEmailConfig().rateLimit).toBe(60);
  });

  it.each([true, false])(
    'saves an edited rate limit while email enabled is %s',
    async (enabled) => {
      loadedEnabled = enabled;
      function Fixture() {
        const state = useAlertDestinationsState({ activeTab: () => 'destinations' });
        const [ready, setReady] = createSignal(false);
        const [saved, setSaved] = createSignal(false);
        onMount(async () => {
          await state.loadDestinations();
          setReady(true);
        });
        return (
          <Show when={ready()}>
            <AlertEmailDestinationsSection
              config={state.emailConfig()}
              setConfig={state.setEmailConfig}
              setHasUnsavedChanges={() => {}}
              onTest={() => {
                throw new Error('No notification test is allowed in this fixture');
              }}
              testing={false}
            />
            <button
              onClick={async () => {
                await state.saveDestinations();
                setSaved(true);
              }}
            >
              Save changes
            </button>
            <Show when={saved()}>Saved</Show>
          </Show>
        );
      }
      render(() => <Fixture />);
      if (!enabled) {
        fireEvent.click(await screen.findByRole('button', { name: 'Show settings' }));
        expect(screen.getByRole('button', { name: 'Send test email' })).toBeDisabled();
      }
      fireEvent.click(await screen.findByRole('button', { name: 'Show advanced options' }));
      const input = screen.getByRole('spinbutton', { name: 'Rate limit' });
      expect(input).toHaveValue(17);
      fireEvent.input(input, { target: { value: '12' } });
      fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
      await waitFor(() => expect(screen.getByText('Saved')).toBeInTheDocument());

      expect(writtenEmailConfig()).toEqual({ ...savedConfig, rateLimit: 12, enabled });
    },
  );
});
