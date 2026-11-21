import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { SSOProvidersPanel } from '../SSOProvidersPanel';
import { setOrgID } from '@/utils/apiClient';

const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('@/stores/notifications', () => ({ notificationStore: notices }));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn(), warn: vi.fn() } }));
const fetchMock = vi.fn<typeof fetch>();
const listURL = '/api/security/sso/providers';
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const provider = (type: 'oidc' | 'saml', enabled: boolean) => ({
  id: `${type}-saved`,
  name: 'Corporate SSO',
  type,
  enabled,
  priority: 0,
  oidcIssuerUrl: 'https://identity.example.test/production',
  samlIdpEntityId: 'urn:synthetic:production',
});
const summary = (type: 'oidc' | 'saml') =>
  type === 'oidc' ? 'https://identity.example.test/production' : 'urn:synthetic:production';
const action = (enabled: boolean) => `${enabled ? 'Disable' : 'Enable'} Corporate SSO`;
const writes = () => fetchMock.mock.calls.filter(([, options]) => options?.method === 'PUT');
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal('fetch', fetchMock);
  setOrgID('sso-saved-state');
  document.cookie = 'pulse_csrf=synthetic-csrf';
});
afterEach(() => {
  cleanup();
  setOrgID(null);
  vi.unstubAllGlobals();
});

describe.each(['oidc', 'saml'] as const)('%s saved provider state', (type) => {
  it.each([true, false])(
    'labels the saved %s flag even when management is unavailable',
    async (enabled) => {
      fetchMock.mockResolvedValue(json({ providers: [provider(type, enabled)] }));
      render(() => <SSOProvidersPanel currentUser="reader" canManage={false} />);
      const toggle = await screen.findByRole('button', { name: action(enabled), pressed: enabled });
      expect(toggle).toBeDisabled();
      expect(toggle).toHaveAccessibleDescription(summary(type));
      expect(
        within(toggle.parentElement!).getByText(enabled ? 'Enabled' : 'Disabled'),
      ).toBeVisible();
      expect(writes()).toHaveLength(0);
    },
  );

  it('does not label a requested enable as saved while its write is pending or refused', async () => {
    const pending = deferred<Response>();
    fetchMock.mockImplementation(async (_url, options) =>
      options?.method === 'PUT' ? pending.promise : json({ providers: [provider(type, false)] }),
    );
    render(() => <SSOProvidersPanel currentUser="admin" />);
    const toggle = await screen.findByRole('button', { name: action(false) });
    await waitFor(() => expect(toggle).toBeEnabled());
    fireEvent.click(toggle);
    await waitFor(() => expect(writes()).toHaveLength(1));
    expect(writes()[0][0]).toBe(`${listURL}/${type}-saved`);
    expect(toggle).toBeDisabled();
    expect(toggle).toHaveAttribute('aria-pressed', 'false');
    expect(within(toggle.parentElement!).getByText('Disabled')).toBeVisible();
    pending.resolve(json({ error: 'SYNTHETIC_PRIVATE_REFUSAL' }, 403));
    await waitFor(() => expect(toggle).toBeEnabled());
    expect(toggle).toHaveAttribute('aria-pressed', 'false');
    expect(within(toggle.parentElement!).getByText('Disabled')).toBeVisible();
    expect(screen.queryByText('Enabled', { exact: true })).not.toBeInTheDocument();
    expect(document.body).not.toHaveTextContent('SYNTHETIC_PRIVATE_REFUSAL');
    expect(writes()).toHaveLength(1);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it('changes the label only from the saved inventory read after an acknowledged toggle', async () => {
    let saved = provider(type, false);
    const readback = deferred<Response>();
    let reads = 0;
    fetchMock.mockImplementation(async (_url, options) => {
      if (options?.method === 'PUT') {
        const body = JSON.parse(options.body as string);
        expect(body).toMatchObject({ id: saved.id, enabled: true });
        expect(body).not.toHaveProperty('oidc');
        expect(body).not.toHaveProperty('saml');
        expect(new Headers(options.headers).get('X-Pulse-Org-ID')).toBe('sso-saved-state');
        expect(new Headers(options.headers).get('X-CSRF-Token')).toBe('synthetic-csrf');
        saved = { ...saved, enabled: true };
        return json({ id: saved.id });
      }
      reads += 1;
      return reads === 1 ? json({ providers: [saved] }) : readback.promise;
    });
    render(() => <SSOProvidersPanel currentUser="admin" />);
    const toggle = await screen.findByRole('button', { name: action(false) });
    await waitFor(() => expect(toggle).toBeEnabled());
    fireEvent.click(toggle);
    await waitFor(() => expect(reads).toBe(2));
    // Loading a saved inventory is not evidence that the requested flag is saved.
    expect(screen.queryByText('Enabled', { exact: true })).not.toBeInTheDocument();
    readback.resolve(json({ providers: [saved] }));
    const confirmed = await screen.findByRole('button', { name: action(true), pressed: true });
    expect(within(confirmed.parentElement!).getByText('Enabled')).toBeVisible();
    expect(confirmed).toHaveAccessibleDescription(summary(type));
    expect(writes()).toHaveLength(1);
  });
});

it('uses saved issuer summaries to distinguish same-named provider switches', async () => {
  fetchMock.mockResolvedValue(
    json({
      providers: [
        provider('oidc', true),
        {
          ...provider('oidc', true),
          id: 'oidc-recovery',
          oidcIssuerUrl: 'https://identity.example.test/recovery',
        },
      ],
    }),
  );
  render(() => <SSOProvidersPanel currentUser="admin" />);
  const toggles = await screen.findAllByRole('button', { name: action(true) });
  expect(toggles).toHaveLength(2);
  expect(toggles[0]).toHaveAccessibleDescription(summary('oidc'));
  expect(toggles[1]).toHaveAccessibleDescription('https://identity.example.test/recovery');
  expect(writes()).toHaveLength(0);
});

it.each([
  {
    inventoryMarker: undefined,
    detailMarker: true,
    description: 'Saved. Blank keeps it.',
  },
  {
    inventoryMarker: true,
    detailMarker: undefined, // The server omits a false marker.
    description: 'No saved secret reported.',
  },
])(
  'describes only the current OIDC detail marker ($detailMarker) and omits a blank secret on save',
  async ({ inventoryMarker, detailMarker, description }) => {
    const saved = provider('oidc', true);
    const detailsURL = `${listURL}/${saved.id}`;
    fetchMock.mockImplementation(async (url, options) => {
      if (options?.method === 'PUT') return json({ id: saved.id });
      return url === detailsURL
        ? json({
            ...saved,
            oidcClientSecretSet: detailMarker,
            oidc: { issuerUrl: saved.oidcIssuerUrl, clientId: 'pulse' },
          })
        : json({ providers: [{ ...saved, oidcClientSecretSet: inventoryMarker }] });
    });
    render(() => <SSOProvidersPanel currentUser="admin" />);
    fireEvent.click(await screen.findByRole('button', { name: 'Edit provider Corporate SSO' }));
    const dialog = await screen.findByRole('dialog', { name: 'Edit OIDC provider' });
    const editor = within(dialog);
    const secret = editor.getByLabelText('Client Secret');
    expect(secret).toHaveAttribute('type', 'password');
    expect(secret).toHaveValue('');
    expect(secret).not.toHaveAttribute('placeholder');
    expect(secret).toHaveAccessibleDescription(description);

    fireEvent.input(editor.getByLabelText('Display Name (Button)'), {
      target: { value: 'Continue with Corporate' },
    });
    fireEvent.submit(dialog.querySelector('form')!);
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(writes()).toHaveLength(1);
    const [url, options] = writes()[0];
    expect(url).toBe(detailsURL);
    const payload = JSON.parse(String(options?.body));
    expect(payload).toMatchObject({ displayName: 'Continue with Corporate' });
    expect(payload.oidc).not.toHaveProperty('clientSecret');
    expect(payload).not.toHaveProperty('oidcClientSecretSet');
  },
);
