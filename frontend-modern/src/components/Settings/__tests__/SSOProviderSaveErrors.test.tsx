import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { SSOProvidersPanel } from '../SSOProvidersPanel';

const fetchMock = vi.fn();
const loggerErrorMock = vi.fn();
const successMock = vi.fn();
vi.mock('@/utils/logger', () => ({
  logger: { error: (...args: unknown[]) => loggerErrorMock(...args), warn: vi.fn() },
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: (...args: unknown[]) => successMock(...args), error: vi.fn() },
}));

const guidance =
  'Could not initialise the sign-in provider. Check its settings, metadata and TLS trust, then try again. No configuration was saved.';
const fallback = 'Unable to save the SSO provider. Check the connection and try saving again.';
const canary = 'SYNTHETIC_PRIVATE_RESPONSE_DETAIL';
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const scenarios = [
  {
    name: 'structured error envelope',
    response: () =>
      json(
        {
          error: guidance,
          code: 'initialization_error',
          details: { private: canary },
          status_code: 400,
          timestamp: 1793050000,
        },
        400,
      ),
    expected: `Unable to save the SSO provider: ${guidance}`,
  },
  {
    name: 'structured message before legacy error',
    response: () =>
      json(
        { message: 'Check the provider settings.', error: canary, details: { private: canary } },
        400,
      ),
    expected: 'Unable to save the SSO provider: Check the provider settings.',
  },
  {
    name: 'plain legacy validation error',
    response: () => new Response('Invalid role mapping', { status: 400 }),
    expected: 'Unable to save the SSO provider: Invalid role mapping',
  },
  {
    name: 'HTML gateway error',
    response: () => new Response(`<html><body>${canary}</body></html>`, { status: 502 }),
    expected: 'Unable to save the SSO provider: Request failed with status 502',
  },
  {
    name: 'malformed JSON',
    response: () => new Response('{"error":', { status: 500 }),
    expected: fallback,
  },
  {
    name: 'oversized message',
    response: () => json({ error: canary.repeat(30) }, 400),
    expected: fallback,
  },
  {
    name: 'lost network response',
    response: () => {
      throw new Error(canary);
    },
    expected: fallback,
  },
  {
    name: 'CSRF refusal without hidden replay',
    response: () => json({ error: 'Refresh your session before saving.' }, 403),
    expected: 'Unable to save the SSO provider: Refresh your session before saving.',
  },
  {
    name: 'rate limit without hidden replay',
    response: () =>
      new Response(JSON.stringify({ error: 'Wait before saving again.' }), {
        status: 429,
        headers: { 'Content-Type': 'application/json', 'Retry-After': '0.001' },
      }),
    expected: 'Unable to save the SSO provider: Wait before saving again.',
  },
];

describe.each(['oidc', 'saml'] as const)('SSO %s save error recovery', (type) => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.stubGlobal('fetch', fetchMock);
    document.cookie = 'pulse_csrf=synthetic-csrf';
  });
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  for (const editing of [false, true]) {
    it.each(scenarios)(`${editing ? 'edit' : 'create'}: $name`, async ({ response, expected }) => {
      const provider = {
        id: 'recovery',
        name: 'Original provider',
        type,
        enabled: true,
        priority: 0,
        allowedGroups: ['Server Access'],
        allowedDomains: ['example.test'],
        groupRoleMappings: { 'Server Access': 'viewer' },
        oidc: {
          issuerUrl: 'https://issuer.example.test',
          clientId: 'pulse',
          clientSecretSet: true,
        },
        saml: { idpEntityId: 'original', idpSsoUrl: 'https://issuer.example.test/sso' },
      };
      let reject = true;
      fetchMock.mockImplementation(async (url: string, options?: RequestInit) => {
        if (options?.method === 'POST' || options?.method === 'PUT')
          return reject ? response() : json({});
        if (url === '/api/security/status')
          return json({ publicUrl: 'https://pulse.example.test' });
        if (url.endsWith('/recovery')) return json(provider);
        return json({ providers: editing ? [provider] : [] });
      });
      render(() => <SSOProvidersPanel />);
      fireEvent.click(
        await screen.findByRole('button', {
          name: editing ? 'Edit provider' : `Add ${type.toUpperCase()}`,
        }),
      );
      const dialog = within(await screen.findByRole('dialog'));
      const name = dialog.getByLabelText('Provider Name') as HTMLInputElement;
      fireEvent.input(name, { target: { value: 'Retained draft' } });
      if (!editing) {
        if (type === 'oidc') {
          fireEvent.input(dialog.getByLabelText('Issuer URL'), {
            target: { value: 'https://issuer.example.test' },
          });
          fireEvent.input(dialog.getByLabelText('Client ID'), { target: { value: 'pulse' } });
        } else {
          fireEvent.input(dialog.getByLabelText('IdP Entity ID'), {
            target: { value: 'attempted' },
          });
          fireEvent.input(dialog.getByLabelText('IdP SSO URL'), {
            target: { value: 'https://issuer.example.test/sso' },
          });
        }
      }
      const save = dialog.getByRole('button', {
        name: editing ? 'Save Changes' : 'Create Provider',
      });
      const form = save.closest('form')!;
      fireEvent.submit(form);
      expect(await dialog.findByRole('alert')).toHaveTextContent(expected);
      expect(dialog.getByRole('alert').textContent).toBe(expected);
      expect(dialog.getByRole('alert')).not.toHaveTextContent(canary);
      expect(name.value).toBe('Retained draft');
      expect(save).toBeEnabled();
      expect(dialog.getByRole('button', { name: 'Cancel' })).toBeEnabled();
      expect(successMock).not.toHaveBeenCalled();
      const writes = () =>
        fetchMock.mock.calls.filter(([, options]) => ['POST', 'PUT'].includes(options?.method));
      expect(writes()).toHaveLength(1);
      expect(JSON.stringify(loggerErrorMock.mock.calls)).not.toContain(canary);
      expect(JSON.stringify(loggerErrorMock.mock.calls)).not.toContain('initialization_error');

      reject = false;
      fireEvent.submit(form);
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      expect(writes()).toHaveLength(2);
      expect(writes()[0][1].body).toBe(writes()[1][1].body);
      if (editing) {
        const body = JSON.parse(writes()[1][1].body);
        expect(body.allowedGroups).toEqual(['Server Access']);
        expect(body.groupRoleMappings).toEqual({ 'Server Access': 'viewer' });
        if (type === 'oidc') expect(body.oidc.clientSecret).toBeUndefined();
      }
      expect(successMock).toHaveBeenCalledOnce();
    });
  }
});
