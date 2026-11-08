import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { SSOProvidersPanel } from '../SSOProvidersPanel';
import type { SSOProvider } from '../ssoProvidersModel';

// Keep the actual cards, dialog, state owner, payload model and authenticated
// client connected. Single-provider/static-read fixtures cannot detect choosing
// a sibling or "saving" a configuration that never survives the next read.
const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('@/stores/notifications', () => ({ notificationStore: notices }));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() } }));

type ProviderType = 'oidc' | 'saml';
type SavedProvider = SSOProvider & {
  oidc?: {
    issuerUrl: string;
    clientId: string;
    redirectUrl: string;
    logoutUrl: string;
    scopes: string[];
  };
  saml?: {
    idpMetadataUrl: string;
    idpEntityId: string;
    idpSsoUrl: string;
    idpCertificate: string;
    spEntityId: string;
    signRequests: boolean;
    allowIdpInitiated: boolean;
    usernameAttr: string;
    emailAttr: string;
    groupsAttr: string;
  };
};
const orgID = 'sso/east?realm=lab#saved';
const apiToken = 'synthetic-sso-administrator-token';
const csrf = 'synthetic-sso-selection-csrf';
const listURL = '/api/security/sso/providers';
const types: ProviderType[] = ['oidc', 'saml'];
const name = 'Company sign-in';
function originalProviders(): SavedProvider[] {
  return ['third', 'first', 'second'].flatMap((part, index) =>
    types.map((type) => ({
      id: `saved/${type}/${part}?realm=lab#identity%雪`,
      name,
      type,
      enabled: true,
      displayName: `${part} ${type} sign-in`,
      priority: index + 7,
      allowedGroups: [`Server Operators ${part}`, `Backup Readers ${part}`],
      allowedDomains: [`${part}.example.test`],
      allowedEmails: [`${part}@example.test`],
      groupsClaim: `${part}_access`,
      groupRoleMappings: {
        [`Server Operators ${part}`]: 'operator',
        [`Backup Readers ${part}`]: 'viewer',
      },
      ...(type === 'oidc'
        ? {
            oidc: {
              issuerUrl: `https://${part}.idp.example.test/realms/fleet`,
              clientId: `${part}-client`,
              redirectUrl: `https://pulse.example.test/callback/${part}`,
              logoutUrl: `https://pulse.example.test/logout/${part}`,
              scopes: ['openid', 'profile', 'email', `${part}_groups`],
            },
          }
        : {
            saml: {
              idpMetadataUrl: `https://${part}.idp.example.test/metadata`,
              idpEntityId: `urn:fixture:${part}`,
              idpSsoUrl: `https://${part}.idp.example.test/sso`,
              idpCertificate: `SYNTHETIC_CERTIFICATE_${part}`,
              spEntityId: `urn:pulse:${part}`,
              signRequests: true,
              allowIdpInitiated: false,
              usernameAttr: `${part}_username`,
              emailAttr: `${part}_email`,
              groupsAttr: `${part}_access`,
            },
          }),
    })),
  );
}
const selected = (type: ProviderType) => originalProviders().filter((p) => p.type === type)[2];
const sibling = (type: ProviderType) => originalProviders().filter((p) => p.type === type)[1];
const summary = (provider: SavedProvider) => provider.oidc?.issuerUrl ?? provider.saml!.idpEntityId;
const pathFor = (provider: SavedProvider) => `${listURL}/${encodeURIComponent(provider.id)}`;
const json = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
}
const fetchMock = vi.fn<typeof fetch>();
const writes = () => fetchMock.mock.calls.filter(([, init]) => (init?.method ?? 'GET') !== 'GET');
const listReads = () =>
  fetchMock.mock.calls.filter(
    ([url, init]) => url === listURL && (init?.method ?? 'GET') === 'GET',
  );
let saved: SavedProvider[];
let secretMarkers: Record<string, string>;
let initialSecrets: Record<string, string>;
let unexpected: string[];
let failListRead: boolean;
let reply: () => Promise<Response>;
let changed: ReturnType<typeof vi.fn>;

function cardFor(provider: SavedProvider) {
  // Locate the existing card header by its visible issuer/entity distinction,
  // never by namesake index, a fixture ID attribute or intended request URL.
  const header = screen.getByText(summary(provider)).parentElement!.parentElement!.parentElement!;
  expect(within(header).getAllByRole('button', { name: 'Edit provider' })).toHaveLength(1);
  return within(header);
}
function expectClientBinding() {
  for (const [url, init] of fetchMock.mock.calls) {
    if (!String(url).startsWith('/api/security/')) continue;
    expect(init).toMatchObject({ credentials: 'include', signal: expect.any(AbortSignal) });
    const headers = new Headers(init?.headers);
    expect(headers.get('X-Pulse-Org-ID')).toBe(orgID);
    expect(headers.get('X-API-Token')).toBe(apiToken);
    for (const internal of ['expectedOrgID', 'retry']) expect(init).not.toHaveProperty(internal);
  }
  for (const [, init] of writes())
    expect(new Headers(init?.headers).get('X-CSRF-Token')).toBe(csrf);
  expect(fetchMock.mock.calls.filter(([url]) => url === '/api/health')).toHaveLength(0);
  expect(getOrgID()).toBe(orgID);
  expect(getApiToken()).toBe(apiToken);
}
function expectedToggle(provider: SavedProvider, enabled: boolean) {
  const {
    id,
    name,
    type,
    displayName,
    priority,
    allowedGroups,
    allowedDomains,
    allowedEmails,
    groupsClaim,
    groupRoleMappings,
  } = provider;
  return {
    id,
    name,
    type,
    enabled,
    displayName,
    priority,
    allowedGroups,
    allowedDomains,
    allowedEmails,
    groupsClaim,
    groupRoleMappings,
  };
}
function expectedSave(provider: SavedProvider) {
  const common = expectedToggle(provider, provider.enabled);
  if (provider.type === 'oidc') return { ...common, oidc: provider.oidc };
  return { ...common, saml: provider.saml };
}
function expectWrite(provider: SavedProvider, method: 'PUT' | 'DELETE', body?: unknown, index = 0) {
  const [url, init] = writes()[index];
  expect(url).toBe(pathFor(provider));
  expect(init?.method).toBe(method);
  expect(init?.body ? JSON.parse(String(init.body)) : undefined).toEqual(body);
  expectClientBinding();
}
function expectPreserved(exceptID?: string) {
  for (const original of originalProviders()) {
    if (original.id === exceptID) continue;
    expect(saved.find((p) => p.id === original.id)).toEqual(original);
  }
  expect(secretMarkers).toEqual(initialSecrets);
}
async function mount() {
  changed = vi.fn();
  render(() => <SSOProvidersPanel currentUser="admin@example.test" onConfigUpdated={changed} />);
  await waitFor(() =>
    expect(screen.getAllByRole('button', { name: 'Edit provider' })).toHaveLength(6),
  );
  await waitFor(() =>
    expect(screen.getAllByRole('button', { name: 'Edit provider' })[0]).toBeEnabled(),
  );
  expect(screen.getAllByText(name, { exact: true })).toHaveLength(6);
  expect(listReads()).toHaveLength(1);
  expect(writes()).toHaveLength(0);
  expectClientBinding();
}
async function edit(provider: SavedProvider) {
  fireEvent.click(cardFor(provider).getByRole('button', { name: 'Edit provider' }));
  const dialog = within(
    await screen.findByRole('dialog', { name: `Edit ${provider.type.toUpperCase()} provider` }),
  );
  const reads = fetchMock.mock.calls.filter(
    ([url, init]) => String(url).startsWith(`${listURL}/`) && (init?.method ?? 'GET') === 'GET',
  );
  expect(reads.at(-1)?.[0]).toBe(pathFor(provider));
  expect(dialog.getByLabelText('Provider Name')).toHaveValue(provider.name);
  expect(dialog.getByLabelText('Display Name (Button)')).toHaveValue(provider.displayName);
  if (provider.type === 'oidc') {
    expect(dialog.getByLabelText('Issuer URL')).toHaveValue(provider.oidc!.issuerUrl);
    expect(dialog.getByLabelText('Client ID')).toHaveValue(provider.oidc!.clientId);
    expect(dialog.getByLabelText('Client Secret')).toHaveValue('');
    expect(dialog.getByLabelText('Redirect URL')).toHaveValue(provider.oidc!.redirectUrl);
    expect(dialog.getByLabelText('Scopes')).toHaveValue(provider.oidc!.scopes.join(' '));
  } else {
    expect(dialog.getByLabelText('IdP Metadata URL')).toHaveValue(provider.saml!.idpMetadataUrl);
    expect(dialog.getByLabelText('IdP Entity ID')).toHaveValue(provider.saml!.idpEntityId);
    expect(dialog.getByLabelText('IdP SSO URL')).toHaveValue(provider.saml!.idpSsoUrl);
    expect(dialog.getByLabelText('IdP Certificate (PEM)')).toHaveValue(provider.saml!.idpCertificate);
    expect(dialog.getByLabelText('Groups Attribute')).toHaveValue(provider.saml!.groupsAttr);
    expect(dialog.getByLabelText('Allow IdP-initiated SSO')).not.toBeChecked();
  }
  fireEvent.click(dialog.getByRole('button', { name: 'Show access restrictions & role mapping' }));
  expect(dialog.getByLabelText('Allowed Groups')).toHaveValue(provider.allowedGroups!.join(', '));
  expect(dialog.getByLabelText('Allowed Domains')).toHaveValue(provider.allowedDomains!.join(', '));
  expect(dialog.getByLabelText('Allowed Emails')).toHaveValue(provider.allowedEmails!.join(', '));
  expect(dialog.getByLabelText('Group Role Mappings')).toHaveValue(
    Object.entries(provider.groupRoleMappings!)
      .map(([group, role]) => `${group}=${role}`)
      .join(', '),
  );
  return dialog;
}
async function checkReconciled(readCount: number, writeCount: number) {
  await waitFor(() => expect(listReads()).toHaveLength(readCount));
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  await waitFor(() =>
    expect(screen.getAllByRole('button', { name: 'Edit provider' })[0]).toBeEnabled(),
  );
  expect(writes()).toHaveLength(writeCount);
  expectClientBinding();
}

beforeEach(() => {
  vi.clearAllMocks();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  vi.stubEnv('MODE', 'production');
  sessionStorage.clear();
  clearAuth();
  setOrgID(orgID);
  setApiToken(apiToken);
  document.cookie = `pulse_csrf=${csrf}; Path=/`;
  saved = originalProviders();
  secretMarkers = Object.fromEntries(
    saved.filter((p) => p.type === 'oidc').map((p) => [p.id, `SYNTHETIC_SERVER_ONLY_${p.id}`]),
  );
  initialSecrets = { ...secretMarkers };
  unexpected = [];
  failListRead = false;
  reply = async () => json({});
  fetchMock.mockImplementation(async (input, init = {}) => {
    const url = String(input);
    const method = init.method ?? 'GET';
    if (method === 'GET') {
      if (url === listURL) {
        if (failListRead) return json({ error: 'synthetic unavailable inventory' }, 503);
        return json({
          providers: saved.map(({ oidc, saml, ...provider }) => ({
            ...provider,
            ...(oidc
              ? { oidcIssuerUrl: oidc.issuerUrl, oidcClientSecretSet: !!secretMarkers[provider.id] }
              : {}),
            ...(saml ? { samlIdpEntityId: saml.idpEntityId } : {}),
          })),
          allowMultipleProviders: true,
        });
      }
      if (url === '/api/security/status') return json({ publicUrl: 'https://pulse.example.test' });
      const provider = saved.find((p) => pathFor(p) === url);
      if (provider)
        return json({
          ...provider,
          ...(provider.oidc
            ? { oidc: { ...provider.oidc, clientSecretSet: !!secretMarkers[provider.id] } }
            : {}),
        });
    }
    const provider = saved.find((p) => pathFor(p) === url);
    if (provider && ['PUT', 'DELETE'].includes(method)) {
      // Apply ONLY the dispatched URL/body, never the test's intended choice.
      // This is synthetic saved-state/readback, not real IdP or server proof.
      if (method === 'DELETE') saved = saved.filter((p) => p.id !== provider.id);
      else {
        const payload = JSON.parse(String(init.body));
        const oidc = payload.oidc;
        if (oidc && Object.hasOwn(oidc, 'clientSecret')) {
          secretMarkers[provider.id] = oidc.clientSecret;
          delete oidc.clientSecret;
        }
        saved = saved.map((p) =>
          p.id === provider.id
            ? {
                ...p,
                ...payload,
                id: p.id,
                ...(oidc ? { oidc: { ...p.oidc, ...oidc } } : {}),
                ...(payload.saml ? { saml: { ...p.saml, ...payload.saml } } : {}),
              }
            : p,
        );
      }
      // Reordering must not rebind a later action to the previous card index.
      saved = [...saved.slice(2), ...saved.slice(0, 2)];
      return reply();
    }
    unexpected.push(`${method} ${url}`);
    throw new Error(`Unexpected synthetic SSO request: ${method} ${url}`);
  });
});
afterEach(() => {
  cleanup();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  expect(unexpected).toEqual([]);
});

describe.each(types)(
  '%s provider selection through the real rendered controls and client',
  (type) => {
    it('saves only the selected namesake unchanged and reopens its complete saved settings', async () => {
      await mount();
      const provider = selected(type);
      const dialog = await edit(provider);
      const completion = deferred<Response>();
      reply = () => completion.promise;
      const form = dialog.getByRole('button', { name: 'Save Changes' }).closest('form')!;
      fireEvent.submit(form);
      await waitFor(() => expect(writes()).toHaveLength(1));
      expectWrite(provider, 'PUT', expectedSave(provider));
      expectPreserved();
      expect(dialog.getByLabelText('Provider Name')).toBeDisabled();
      expect(dialog.getByRole('button', { name: 'Cancel' })).toBeDisabled();
      fireEvent.submit(form);
      expect(writes()).toHaveLength(1);
      expect(changed).not.toHaveBeenCalled();
      completion.resolve(json({}));
      await checkReconciled(2, 1);
      expect(changed).toHaveBeenCalledOnce();
      expect(notices.success).toHaveBeenCalledExactlyOnceWith('SSO provider has been updated.');
      expect(notices.error).not.toHaveBeenCalled();
      await edit(provider);
      expectPreserved();
    });

    it('discards a sibling draft before saving only the selected namesake restrictions', async () => {
      await mount();
      const cancelled = await edit(sibling(type));
      fireEvent.input(cancelled.getByLabelText('Provider Name'), {
        target: { value: 'Discarded name' },
      });
      fireEvent.input(cancelled.getByLabelText('Group Role Mappings'), {
        target: { value: 'Discarded Group=admin' },
      });
      if (type === 'oidc')
        fireEvent.input(cancelled.getByLabelText('Client Secret'), {
          target: { value: 'SYNTHETIC_DISCARDED_DRAFT_SECRET' },
        });
      fireEvent.click(cancelled.getByRole('button', { name: 'Cancel' }));
      expect(screen.queryByRole('dialog')).toBeNull();
      expect(writes()).toHaveLength(0);
      const provider = selected(type);
      const dialog = await edit(provider);
      const next = {
        ...provider,
        name: 'Selected saved sign-in',
        allowedGroups: ['Fleet Operators East', 'Backup Readers second'],
        groupRoleMappings: {
          'Fleet Operators East': 'operator',
          'Backup Readers second': 'viewer',
        },
      };
      fireEvent.input(dialog.getByLabelText('Provider Name'), {
        target: { value: ` ${next.name} ` },
      });
      fireEvent.input(dialog.getByLabelText('Allowed Groups'), {
        target: { value: 'Fleet Operators East,\nBackup Readers second' },
      });
      fireEvent.input(dialog.getByLabelText('Group Role Mappings'), {
        target: { value: 'Fleet Operators East=operator,\nBackup Readers second=viewer' },
      });
      fireEvent.submit(dialog.getByRole('button', { name: 'Save Changes' }).closest('form')!);
      await checkReconciled(2, 1);
      expectWrite(provider, 'PUT', expectedSave(next));
      expect(saved.find((p) => p.id === provider.id)).toEqual(next);
      expectPreserved(provider.id);
      await edit(next);
      expect(writes()).toHaveLength(1);
    });

    it('disables and re-enables only the selected namesake after reordered saved reads', async () => {
      await mount();
      const provider = selected(type);
      for (const [index, enabled] of [false, true].entries()) {
        const completion = deferred<Response>();
        reply = () => completion.promise;
        const toggle = cardFor(provider).getByRole('button', {
          name: `${enabled ? 'Enable' : 'Disable'} ${name}`,
        });
        fireEvent.click(toggle);
        await waitFor(() => expect(writes()).toHaveLength(index + 1));
        expectWrite(provider, 'PUT', expectedToggle(provider, enabled), index);
        expect(toggle).toBeDisabled();
        fireEvent.click(toggle);
        expect(writes()).toHaveLength(index + 1);
        expectPreserved(provider.id);
        expect(saved.find((p) => p.id === provider.id)).toEqual({ ...provider, enabled });
        completion.resolve(json({}));
        await checkReconciled(index + 2, index + 1);
        expect(
          cardFor(provider).getByRole('button', {
            name: `${enabled ? 'Disable' : 'Enable'} ${name}`,
          }),
        ).toHaveAttribute('aria-pressed', String(enabled));
        expect(changed).toHaveBeenCalledTimes(index + 1);
      }
      await edit(provider);
      expectPreserved();
    });

    it('cancels a sibling deletion then removes only the confirmed selected namesake', async () => {
      await mount();
      fireEvent.click(cardFor(sibling(type)).getByRole('button', { name: 'Delete provider' }));
      const cancelled = within(await screen.findByRole('dialog', { name: 'Delete provider' }));
      fireEvent.click(cancelled.getByRole('button', { name: 'Cancel' }));
      expect(writes()).toHaveLength(0);
      expectPreserved();
      const provider = selected(type);
      fireEvent.click(cardFor(provider).getByRole('button', { name: 'Delete provider' }));
      const confirmation = within(await screen.findByRole('dialog', { name: 'Delete provider' }));
      const completion = deferred<Response>();
      reply = () => completion.promise;
      const remove = confirmation.getByRole('button', { name: 'Delete Provider' });
      fireEvent.click(remove);
      await waitFor(() => expect(writes()).toHaveLength(1));
      expectWrite(provider, 'DELETE');
      expect(remove).toBeDisabled();
      expect(confirmation.getByRole('button', { name: 'Cancel' })).toBeDisabled();
      fireEvent.click(remove);
      expect(writes()).toHaveLength(1);
      completion.resolve(json({}));
      await checkReconciled(2, 1);
      expect(screen.queryByText(summary(provider))).toBeNull();
      expect(saved).toHaveLength(5);
      expectPreserved(provider.id);
      for (const kept of saved)
        expect(cardFor(kept).getByRole('button', { name: 'Edit provider' })).toBeEnabled();
      expect(notices.success).toHaveBeenCalledExactlyOnceWith('SSO provider has been removed.');
      expect(changed).toHaveBeenCalledOnce();
    });

    it('recovers an applied selected edit with a lost reply by reading, never resubmitting', async () => {
      await mount();
      const provider = selected(type);
      const dialog = await edit(provider);
      const next = { ...provider, name: 'Saved before reply was lost' };
      fireEvent.input(dialog.getByLabelText('Provider Name'), { target: { value: next.name } });
      reply = async () => json({ error: 'synthetic gateway reply loss' }, 502);
      fireEvent.submit(dialog.getByRole('button', { name: 'Save Changes' }).closest('form')!);
      const recovery = await dialog.findByRole('button', {
        name: 'Close draft and check providers',
      });
      expect(await dialog.findByRole('alert')).toHaveTextContent('may already have been applied');
      expect(listReads()).toHaveLength(1);
      expect(writes()).toHaveLength(1);
      expectWrite(provider, 'PUT', expectedSave(next));
      expect(dialog.getByLabelText('Provider Name')).toBeDisabled();
      expect(notices.success).not.toHaveBeenCalled();
      expect(changed).not.toHaveBeenCalled();
      failListRead = true;
      fireEvent.click(recovery);
      await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
      expect(await screen.findByText(/Configured providers are unknown\./)).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Add OIDC' })).toBeDisabled();
      expect(writes()).toHaveLength(1);
      failListRead = false;
      fireEvent.click(screen.getByRole('button', { name: 'Check saved providers' }));
      await checkReconciled(3, 1);
      expect(screen.queryByText(/may already have been applied/)).toBeNull();
      expect(saved.find((p) => p.id === provider.id)).toEqual(next);
      expectPreserved(provider.id);
      await edit(next);
      expect(writes()).toHaveLength(1);
      expect(notices.success).not.toHaveBeenCalled();
      expect(changed).not.toHaveBeenCalled();
    });
  },
);
