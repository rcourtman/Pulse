import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { SSOProvidersPanel } from '../SSOProvidersPanel';
import { setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';

const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
const logs = vi.hoisted(() => ({ error: vi.fn(), warn: vi.fn() }));
vi.mock('@/stores/notifications', () => ({ notificationStore: notices }));
vi.mock('@/utils/logger', () => ({ logger: logs }));
const fetchMock = vi.fn();
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const provider = (type: 'oidc' | 'saml') => ({
  id: 'saved/provider',
  name: 'Saved provider',
  type,
  enabled: true,
  priority: 0,
  allowedGroups: ['Server Access'],
  allowedDomains: ['example.test'],
  groupRoleMappings: { 'Server Access': 'viewer' },
  oidc: { issuerUrl: 'https://issuer.example.test', clientId: 'pulse', clientSecretSet: true },
  saml: { idpEntityId: 'saved', idpSsoUrl: 'https://issuer.example.test/sso' },
});
const writes = () =>
  fetchMock.mock.calls.filter(([, options]) => options?.method && options.method !== 'GET');
const reads = () =>
  fetchMock.mock.calls.filter(
    ([url, options]) => url === '/api/security/sso/providers' && !options?.method,
  );

beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal('fetch', fetchMock);
  document.cookie = 'pulse_csrf=synthetic-sso-csrf';
  setOrgID('sso-a');
});
afterEach(() => {
  cleanup();
  setOrgID(null);
  vi.unstubAllGlobals();
});

for (const type of ['oidc', 'saml'] as const) {
  describe(`${type} uncertain provider changes`, () => {
    for (const operation of ['create', 'edit', 'toggle', 'delete'] as const) {
      it.each(['network', 'gateway', 'server'] as const)(
        `${operation}: checks saved state after a %s failure without replay`,
        async (failure) => {
          let saved: ReturnType<typeof provider> | null =
            operation === 'create' ? null : provider(type);
          let failNextRead = false;
          fetchMock.mockImplementation(async (url: string, options?: RequestInit) => {
            if (options?.method && options.method !== 'GET') {
              const body = options.body ? JSON.parse(String(options.body)) : {};
              saved =
                operation === 'delete'
                  ? null
                  : { ...(saved ?? provider(type)), ...body, id: 'saved/provider' };
              if (failure === 'network') throw new Error('PRIVATE_PROVIDER_FAILURE');
              if (failure === 'gateway')
                return new Response('<html>PRIVATE_PROVIDER_FAILURE</html>', { status: 502 });
              return json({ error: 'PRIVATE_PROVIDER_FAILURE' }, 500);
            }
            if (url === '/api/security/status')
              return json({ publicUrl: 'https://pulse.example.test' });
            if (url === '/api/security/sso/providers') {
              if (failNextRead) {
                failNextRead = false;
                return json({ providers: [{}] });
              }
              return json({ providers: saved ? [saved] : [] });
            }
            return json(saved);
          });
          const changed = vi.fn();
          render(() => <SSOProvidersPanel currentUser="admin" onConfigUpdated={changed} />);
          const add = await screen.findByRole('button', { name: 'Add OIDC' });
          await waitFor(() => expect(add).toBeEnabled());
          if (operation === 'create' || operation === 'edit') {
            fireEvent.click(
              screen.getByRole('button', {
                name: operation === 'create' ? `Add ${type.toUpperCase()}` : 'Edit provider',
              }),
            );
            const dialog = within(await screen.findByRole('dialog'));
            fireEvent.input(dialog.getByLabelText('Provider Name'), {
              target: { value: 'Changed provider' },
            });
            if (operation === 'create') {
              if (type === 'oidc') {
                fireEvent.input(dialog.getByLabelText('Issuer URL'), {
                  target: { value: 'https://issuer.example.test' },
                });
                fireEvent.input(dialog.getByLabelText('Client ID'), { target: { value: 'pulse' } });
              } else {
                fireEvent.input(dialog.getByLabelText('IdP Entity ID'), {
                  target: { value: 'saved' },
                });
                fireEvent.input(dialog.getByLabelText('IdP SSO URL'), {
                  target: { value: 'https://issuer.example.test/sso' },
                });
              }
            }
            const save = dialog.getByRole('button', {
              name: operation === 'create' ? 'Create Provider' : 'Save Changes',
            });
            const form = save.closest('form')!;
            fireEvent.submit(form);
            expect(await dialog.findByRole('alert')).toHaveTextContent(
              'may already have been applied',
            );
            expect(dialog.getByLabelText('Provider Name')).toHaveValue('Changed provider');
            expect(save).toBeDisabled();
            fireEvent.submit(form);
          } else if (operation === 'delete') {
            fireEvent.click(screen.getByRole('button', { name: 'Delete provider' }));
            const confirm = within(screen.getByRole('dialog')).getByRole('button', {
              name: 'Delete Provider',
            });
            fireEvent.click(confirm);
            expect(await within(screen.getByRole('dialog')).findByRole('alert')).toHaveTextContent(
              'may already have been applied',
            );
            expect(confirm).toBeDisabled();
            fireEvent.click(confirm);
          } else {
            const toggle = screen.getByRole('button', { name: 'Disable Saved provider' });
            fireEvent.click(toggle);
            expect(await screen.findByRole('alert')).toHaveTextContent(
              'may already have been applied',
            );
            expect(toggle).toBeDisabled();
            fireEvent.click(toggle);
            expect(screen.getByText('Saved provider')).toBeInTheDocument();
            expect(screen.getByRole('alert')).toHaveTextContent('last successful read');
          }
          expect(writes()).toHaveLength(1);
          expect(reads()).toHaveLength(1);
          expect(writes()[0][1].headers['X-Pulse-Org-ID']).toBe('sso-a');
          expect(notices.success).not.toHaveBeenCalled();
          expect(changed).not.toHaveBeenCalled();
          expect(document.body).not.toHaveTextContent('PRIVATE_PROVIDER_FAILURE');
          expect(JSON.stringify(logs.error.mock.calls)).not.toContain('PRIVATE_PROVIDER_FAILURE');

          failNextRead = true;
          const recoveryScope = screen.queryByRole('dialog')
            ? within(screen.getByRole('dialog'))
            : screen;
          fireEvent.click(recoveryScope.getByRole('button', { name: 'Check saved providers' }));
          await screen.findByText(
            'Unable to load SSO providers. Configured providers are unknown.',
          );
          expect(screen.queryByRole('dialog')).toBeNull();
          expect(screen.queryByText('No SSO providers configured')).toBeNull();
          expect(screen.getByRole('button', { name: 'Add OIDC' })).toBeDisabled();
          expect(reads()).toHaveLength(2);
          expect(writes()).toHaveLength(1);
          fireEvent.click(screen.getByRole('button', { name: 'Check saved providers' }));
          await waitFor(() =>
            expect(screen.getByRole('button', { name: 'Add OIDC' })).toBeEnabled(),
          );
          expect(reads()).toHaveLength(3);
          expect(writes()).toHaveLength(1);
          expect(document.body).not.toHaveTextContent('could not be confirmed');
          if (operation === 'delete')
            expect(screen.getByText('No SSO providers configured')).toBeInTheDocument();
          else if (operation === 'toggle')
            expect(screen.getByRole('button', { name: 'Enable Saved provider' })).toHaveAttribute(
              'aria-pressed',
              'false',
            );
          else {
            expect(screen.getByText('Changed provider')).toBeInTheDocument();
            fireEvent.click(screen.getByRole('button', { name: 'Edit provider' }));
            const editor = within(await screen.findByRole('dialog'));
            expect(editor.getByLabelText('Provider Name')).toHaveValue('Changed provider');
            if (type === 'oidc') {
              expect(editor.getByLabelText('Client Secret')).toHaveValue('');
              expect(JSON.parse(writes()[0][1].body).oidc.clientSecret).toBeUndefined();
            }
            if (operation === 'edit') {
              expect(JSON.parse(writes()[0][1].body).allowedGroups).toEqual(['Server Access']);
              expect(JSON.parse(writes()[0][1].body).groupRoleMappings).toEqual({
                'Server Access': 'viewer',
              });
            }
          }
          expect(notices.success).not.toHaveBeenCalled();
          expect(changed).not.toHaveBeenCalled();
        },
      );
    }
  });
}

it.each(['toggle', 'delete'] as const)(
  'keeps a definitely refused %s correctable without inventing uncertainty',
  async (operation) => {
    fetchMock.mockImplementation(async (url: string, options?: RequestInit) => {
      if (options?.method) return json({ error: 'Refused' }, 409);
      if (url === '/api/security/status') return json({ publicUrl: 'https://pulse.example.test' });
      return json({ providers: [provider('oidc')] });
    });
    render(() => <SSOProvidersPanel />);
    await waitFor(() => expect(screen.getByRole('button', { name: 'Add OIDC' })).toBeEnabled());
    if (operation === 'delete') {
      fireEvent.click(screen.getByRole('button', { name: 'Delete provider' }));
      fireEvent.click(
        within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete Provider' }),
      );
    } else fireEvent.click(screen.getByRole('button', { name: 'Disable Saved provider' }));
    await waitFor(() => expect(notices.error).toHaveBeenCalledOnce());
    expect(screen.queryByRole('button', { name: 'Check saved providers' })).toBeNull();
    expect(writes()).toHaveLength(1);
    expect(reads()).toHaveLength(1);
  },
);

it('does not carry an uncertain reader or its draft into a new organization', async () => {
  let resolve!: (response: Response) => void;
  const pending = new Promise<Response>((done) => {
    resolve = done;
  });
  fetchMock.mockImplementation(async (url: string, options?: RequestInit) => {
    if (options?.method) return pending;
    if (url === '/api/security/status') return json({ publicUrl: 'https://pulse.example.test' });
    return json({ providers: [provider('oidc')] });
  });
  const [user, setUser] = createSignal('admin');
  render(() => <SSOProvidersPanel currentUser={user()} />);
  await screen.findByText('Saved provider');
  fireEvent.click(screen.getByRole('button', { name: 'Disable Saved provider' }));
  setOrgID('sso-b');
  setUser('reader');
  eventBus.emit('org_switched', 'sso-b');
  await waitFor(() => expect(reads().length).toBeGreaterThan(1));
  resolve(json({ error: 'PRIVATE_PROVIDER_FAILURE' }, 500));
  await new Promise((done) => setTimeout(done, 0));
  expect(document.body).not.toHaveTextContent('could not be confirmed');
  expect(writes()).toHaveLength(1);
  expect(writes()[0][1].signal.aborted).toBe(true);
  expect(notices.error).not.toHaveBeenCalled();
});
