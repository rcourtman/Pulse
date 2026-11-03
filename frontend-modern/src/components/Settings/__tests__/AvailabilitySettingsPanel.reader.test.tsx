import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { eventBus } from '@/stores/events';
import { clearAuth, setApiToken, setOrgID } from '@/utils/apiClient';
import { AvailabilitySettingsPanel } from '../AvailabilitySettingsPanel';

vi.mock('@solidjs/router', async () => ({
  ...(await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router')),
  useLocation: () => ({ pathname: '/settings/monitoring/availability', search: '' }),
  useNavigate: () => vi.fn(),
}));
vi.mock('@/hooks/useResources', () => ({ useResources: () => ({ resources: () => [] }) }));
vi.mock('@/stores/license', () => ({
  hasFeature: () => false,
  runtimeCapabilitiesLoaded: () => true,
  loadRuntimeCapabilities: async () => undefined,
}));
const fetchMock = vi.fn<typeof fetch>();
const target = (name = 'Private check') => ({
  id: 'same-id',
  name,
  address: 'service.example.invalid',
  protocol: 'https',
  http: {
    method: 'GET',
    expectedStatusMin: 200,
    expectedStatusMax: 399,
    authentication: { type: 'basic', username: 'monitor' },
  },
  httpSecrets: { passwordConfigured: true, bodyConfigured: false, bearerTokenConfigured: false },
  enabled: true,
});
const json = (data: unknown, status = 200) =>
  new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json' } });
const delayed = () => {
  let release!: (value: unknown) => void;
  const response = new Response(
    new ReadableStream<Uint8Array>({
      start(controller) {
        release = (value) => {
          controller.enqueue(new TextEncoder().encode(JSON.stringify(value)));
          controller.close();
        };
      },
    }),
    { status: 200 },
  );
  return { response, release };
};
const ready = (name = 'Private check') =>
  waitFor(() => expect(screen.getByText(name)).toBeInTheDocument());

beforeEach(() => {
  sessionStorage.clear();
  localStorage.clear();
  clearAuth();
  setOrgID('north');
  setApiToken('synthetic-token');
  document.cookie = 'pulse_csrf=fixture-token; Path=/';
  fetchMock.mockReset().mockImplementation(async () => json([target('Replacement check')]));
  fetchMock.mockResolvedValueOnce(json([target()]));
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => {
  cleanup();
  clearAuth();
  setOrgID(null);
  vi.unstubAllGlobals();
});

describe('Availability checks retain one reader through the actual API transport', () => {
  it.each([
    'organisation',
    'principal round trip',
    'capability',
    'credential',
    'disposal',
    'storage drift',
  ] as const)('withdraws inventory, secret draft and late JSON after %s', async (change) => {
    const [user, setUser] = createSignal('reader-a');
    const [allowed, setAllowed] = createSignal(true);
    const [access, setAccess] = createSignal('hint-a');
    const view = render(() => (
      <AvailabilitySettingsPanel currentUser={user()} canRead={allowed()} accessKey={access()} />
    ));
    await ready();
    fireEvent.click(screen.getByRole('button', { name: 'Manage' }));
    await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Replacement check'));
    fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Unsaved private draft' } });
    fireEvent.input(screen.getByLabelText('Password'), {
      target: { value: 'Synthetic unsaved secret' },
    });
    const old = delayed();
    fetchMock.mockResolvedValueOnce(old.response);
    // Refresh remains available behind the dialog; a same-reader refresh does not discard its draft.
    fireEvent.click(screen.getByRole('button', { name: 'Refresh', exact: true }));
    await waitFor(() => expect(old.response.body?.locked).toBe(true));
    const options = fetchMock.mock.calls.at(-1)![1]!;
    expect(new Headers(options.headers).get('X-Pulse-Org-ID')).toBe('north');
    if (change === 'organisation') {
      setOrgID('south');
      eventBus.emit('org_switched', 'south');
    }
    if (change === 'principal round trip') {
      setUser('reader-b');
      await ready('Replacement check');
      setUser('reader-a');
    }
    if (change === 'capability') setAllowed(false);
    if (change === 'credential') setAccess('hint-b');
    if (change === 'disposal') view.unmount();
    if (change === 'storage drift') setOrgID('south');
    old.release([target('Obsolete private response')]);
    await waitFor(() => expect(options.signal?.aborted).toBe(true));
    expect(screen.queryByText('Obsolete private response')).not.toBeInTheDocument();
    expect(screen.queryByDisplayValue('Unsaved private draft')).not.toBeInTheDocument();
    expect(screen.queryByDisplayValue('Synthetic unsaved secret')).not.toBeInTheDocument();
    expect(screen.queryByText('Private check')).not.toBeInTheDocument();
    if (change === 'storage drift') {
      expect(screen.getByText(/session or organisation changed/)).toBeInTheDocument();
    } else if (!['capability', 'disposal'].includes(change)) await ready('Replacement check');
  });

  it('preserves a same-reader draft across copied props, but cancels an editor test on close', async () => {
    const [user, setUser] = createSignal('reader-a');
    render(() => <AvailabilitySettingsPanel currentUser={user()} />);
    await ready();
    fireEvent.click(screen.getByRole('button', { name: 'Manage' }));
    await waitFor(() => expect(screen.getByLabelText('Name')).toBeInTheDocument());
    fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Live draft' } });
    setUser('reader-a');
    expect(screen.getByLabelText('Name')).toHaveValue('Live draft');
    const old = delayed();
    fetchMock.mockResolvedValueOnce(old.response);
    fireEvent.click(screen.getByRole('button', { name: 'Test probe' }));
    await waitFor(() => expect(old.response.body?.locked).toBe(true));
    const options = fetchMock.mock.calls.at(-1)![1]!;
    fireEvent.click(screen.getByRole('button', { name: 'Close availability check dialog' }));
    expect(options.signal?.aborted).toBe(true);
    old.release({ success: true, latencyMillis: 999 });
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByText(/999 ms/)).not.toBeInTheDocument();
  });

  it.each([null, {}, [target(), target()], [{ ...target(), enabled: 'yes' }]])(
    'does not turn a malformed or unknown inventory into onboarding: %j',
    async (data) => {
      fetchMock.mockReset().mockResolvedValueOnce(json(data));
      render(() => <AvailabilitySettingsPanel />);
      await waitFor(() =>
        expect(screen.getByRole('alert')).toHaveTextContent('could not be loaded'),
      );
      expect(screen.queryByText('No availability checks yet')).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Add service/device check' })).toBeDisabled();
      fetchMock.mockResolvedValueOnce(json([]));
      fireEvent.click(screen.getByRole('button', { name: 'Retry', exact: true }));
      await waitFor(() =>
        expect(screen.getByText('No availability checks yet')).toBeInTheDocument(),
      );
      expect(fetchMock).toHaveBeenCalledTimes(2);
      expect(fetchMock.mock.calls.every(([, init]) => !init?.method || init.method === 'GET')).toBe(
        true,
      );
    },
  );

  it.each([403])(
    'withdraws accepted rows and the editor on HTTP %i; recovery is only one read',
    async (status) => {
      render(() => <AvailabilitySettingsPanel />);
      await ready();
      fireEvent.click(screen.getByRole('button', { name: 'Manage' }));
      await waitFor(() => expect(screen.getByLabelText('Name')).toBeInTheDocument());
      fetchMock.mockResolvedValueOnce(json({ message: 'PRIVATE_DENIAL_DO_NOT_ECHO' }, status));
      fireEvent.click(screen.getByRole('button', { name: 'Refresh', exact: true }));
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      expect(screen.queryByText('Private check')).not.toBeInTheDocument();
      expect(screen.queryByText(/PRIVATE_DENIAL/)).not.toBeInTheDocument();
      const count = fetchMock.mock.calls.length;
      fetchMock.mockResolvedValueOnce(json([target('Recovered check')]));
      fireEvent.click(screen.getByRole('button', { name: 'Retry', exact: true }));
      await ready('Recovered check');
      expect(fetchMock.mock.calls.length).toBe(count + 1);
    },
  );

  it('retains accepted rows on an unavailable refresh but blocks actions until read-only recovery', async () => {
    render(() => <AvailabilitySettingsPanel />);
    await ready();
    fetchMock.mockResolvedValueOnce(json({}, 503));
    fireEvent.click(screen.getByRole('button', { name: 'Refresh', exact: true }));
    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());
    expect(screen.getByText('Private check')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Pause' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Manage' })).toBeDisabled();
  });

  it.each(['Pause', 'Test'] as const)(
    'serialises %s and rejects its obsolete completion without a refetch',
    async (action) => {
      const [user, setUser] = createSignal('reader-a');
      render(() => <AvailabilitySettingsPanel currentUser={user()} />);
      await ready();
      const old = delayed();
      fetchMock.mockResolvedValueOnce(old.response);
      fireEvent.click(screen.getByRole('button', { name: action, exact: true }));
      await waitFor(() => expect(old.response.body?.locked).toBe(true));
      const options = fetchMock.mock.calls.at(-1)![1]!;
      expect(screen.getByRole('button', { name: 'Manage' })).toBeDisabled();
      setUser('reader-b');
      await ready('Replacement check');
      const count = fetchMock.mock.calls.length;
      old.release({ success: true, latencyMillis: 666 });
      await new Promise((resolve) => setTimeout(resolve, 0));
      expect(options.signal?.aborted).toBe(true);
      expect(fetchMock.mock.calls.length).toBe(count);
      expect(screen.queryByText('Private check')).not.toBeInTheDocument();
    },
  );

  it('keeps a missing saved check uneditable and retries only its configuration read', async () => {
    render(() => <AvailabilitySettingsPanel />);
    await ready();
    fetchMock.mockResolvedValueOnce(json([]));
    fireEvent.click(screen.getByRole('button', { name: 'Manage' }));
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('could not be loaded'));
    expect(screen.queryByLabelText('Name')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Save target' })).not.toBeInTheDocument();
    fetchMock.mockResolvedValueOnce(json([target()]));
    fireEvent.click(screen.getByRole('button', { name: 'Retry saved check' }));
    await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Private check'));
    expect(fetchMock.mock.calls.every(([, init]) => !init?.method || init.method === 'GET')).toBe(
      true,
    );
  });
});

it('cannot finish or refetch an obsolete saved editor write in a replacement reader', async () => {
  const [user, setUser] = createSignal('reader-a');
  render(() => <AvailabilitySettingsPanel currentUser={user()} />);
  await ready();
  fireEvent.click(screen.getByRole('button', { name: 'Manage' }));
  await waitFor(() => expect(screen.getByLabelText('Name')).toBeInTheDocument());
  fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Old saved draft' } });
  const old = delayed();
  fetchMock.mockResolvedValueOnce(old.response);
  fireEvent.click(screen.getByRole('button', { name: 'Save target' }));
  await waitFor(() => expect(old.response.body?.locked).toBe(true));
  const options = fetchMock.mock.calls.at(-1)![1]!;
  expect(JSON.parse(options.body as string).name).toBe('Old saved draft');
  setUser('reader-b');
  await ready('Replacement check');
  const count = fetchMock.mock.calls.length;
  old.release(target('Old saved draft'));
  await new Promise((resolve) => setTimeout(resolve, 0));
  expect(options.signal?.aborted).toBe(true);
  expect(fetchMock.mock.calls.length).toBe(count);
  expect(screen.queryByDisplayValue('Old saved draft')).not.toBeInTheDocument();
});

it('requires two current removal clicks and never repeats a refused DELETE', async () => {
  render(() => <AvailabilitySettingsPanel />);
  await ready();
  fireEvent.click(screen.getByRole('button', { name: 'Manage' }));
  await waitFor(() => expect(screen.getByLabelText('Name')).toBeInTheDocument());
  const start = fetchMock.mock.calls.length;
  fireEvent.click(screen.getByRole('button', { name: 'Remove target' }));
  expect(fetchMock.mock.calls.length).toBe(start);
  fetchMock.mockResolvedValueOnce(json({ message: 'PRIVATE_REMOVAL_REFUSAL' }, 403));
  fireEvent.click(screen.getByRole('button', { name: 'Click again to confirm' }));
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  expect(fetchMock.mock.calls.length).toBe(start + 1);
  expect(fetchMock.mock.calls.at(-1)?.[1]?.method).toBe('DELETE');
  expect(screen.queryByText('Private check')).not.toBeInTheDocument();
  expect(screen.queryByText(/PRIVATE_REMOVAL_REFUSAL/)).not.toBeInTheDocument();
});

it('withdraws without a fallback read when 401 clears the admitted auth context', async () => {
  render(() => <AvailabilitySettingsPanel />);
  await ready();
  const start = fetchMock.mock.calls.length;
  fetchMock.mockResolvedValueOnce(json({ message: 'PRIVATE_AUTH_FAILURE' }, 401));
  fireEvent.click(screen.getByRole('button', { name: 'Refresh', exact: true }));
  await waitFor(() =>
    expect(screen.getByText(/session or organisation changed/)).toBeInTheDocument(),
  );
  expect(screen.queryByText('Private check')).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: 'Refresh', exact: true })).not.toBeInTheDocument();
  expect(fetchMock.mock.calls.length).toBe(start + 1);
  expect(screen.queryByText(/PRIVATE_AUTH_FAILURE/)).not.toBeInTheDocument();
});
