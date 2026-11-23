import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { SSOProvidersPanel } from '../SSOProvidersPanel';

const fetchMock = vi.fn();
const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('@/stores/notifications', () => ({ notificationStore: notices }));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn(), warn: vi.fn() } }));
const json = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
const metadata = 'https://pulse.example.test/base/api/saml/setup/metadata/?view=a%2fb';
const acs = 'https://pulse.example.test/base/api/saml/setup/acs/?tenant=ops%2fwest';
const saml = {
  id: 'setup',
  name: 'Operations',
  type: 'saml' as const,
  enabled: true,
  priority: 0,
  samlMetadataUrl: metadata,
  samlAcsUrl: acs,
};
const detail = {
  ...saml,
  saml: {
    idpSsoUrl: 'https://idp.example.test/sso',
    idpEntityId: 'urn:example:operations',
    spEntityId: 'urn:pulse:operations',
  },
  allowedGroups: ['Operators'],
  groupRoleMappings: { Operators: 'viewer' },
};
const setup = (providers: unknown[], details: unknown = detail) => {
  fetchMock.mockImplementation(async (url: string) => {
    if (url === '/api/security/sso/providers') return json({ providers });
    if (url === '/api/security/sso/providers/setup') return json(details);
    // This is deliberately plausible but not the registration authority.
    if (url === '/api/security/status') return json({ publicUrl: 'https://wrong.example.test' });
    throw new Error('Unexpected request');
  });
};
const edit = async (type = 'SAML') => {
  const button = await screen.findByRole('button', { name: /^Edit provider / });
  await waitFor(() => expect(button).toBeEnabled());
  fireEvent.click(button);
  return within(await screen.findByRole('dialog', { name: `Edit ${type} provider` }));
};
const readOnlyRequests = () => {
  expect(fetchMock.mock.calls.every(([, options]) => !options?.method)).toBe(true);
  expect(fetchMock.mock.calls.map(([url]) => url)).not.toContain('/api/security/status');
};

beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal('fetch', fetchMock);
  document.cookie = 'pulse_csrf=synthetic-registration-test';
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('SSO sign-in restrictions and role mapping guidance', () => {
  it.each([
    { type: 'oidc', groups: [] },
    { type: 'saml', groups: ['Fleet Operators', 'Service Reviewers'] },
  ] as const)('describes each $type filter even when one is blank', async ({ type, groups }) => {
    const provider = { ...saml, type };
    setup([provider], {
      ...provider,
      allowedGroups: groups,
      allowedDomains: ['example.test'],
      allowedEmails: ['operator@example.test'],
      groupRoleMappings: { 'Fleet Operators': 'operator' },
    });
    render(() => <SSOProvidersPanel />);

    const dialog = await edit(type.toUpperCase());
    expect(dialog.queryByRole('textbox', { name: 'Allowed Groups' })).toBeNull();
    fireEvent.click(
      dialog.getByRole('button', { name: 'Show access restrictions & role mapping' }),
    );
    const rule = 'Each filled list needs a match. Blank skips only that check.';
    const allowedGroups = dialog.getByRole('textbox', { name: 'Allowed Groups' });
    const allowedDomains = dialog.getByRole('textbox', { name: 'Allowed Domains' });
    const allowedEmails = dialog.getByRole('textbox', { name: 'Allowed Emails' });
    expect(allowedGroups).toHaveValue(groups.join(', '));
    expect(allowedDomains).toHaveValue('example.test');
    expect(allowedEmails).toHaveValue('operator@example.test');
    expect(allowedGroups).toHaveAccessibleDescription(
      `${rule} Use commas or new lines. Spaces are fine. Double-quote names with commas or =.`,
    );
    expect(allowedDomains).toHaveAccessibleDescription(`${rule} Email domains (without @)`);
    expect(allowedEmails).toHaveAccessibleDescription(rule);
    expect(
      dialog.getByRole('textbox', { name: 'Group Role Mappings' }),
    ).toHaveAccessibleDescription(
      'Roles apply after sign-in checks. Use group=roleId, e.g. "Ops, East"=viewer. Commas or new lines.',
    );
    expect(dialog.queryByText(/Empty allows all/)).toBeNull();
    readOnlyRequests();
  });
});

describe('SSO editor registration authority', () => {
  it.each([metadata, 'https://pulse.lan:8443/api/saml/setup/metadata?view=full'])(
    'displays and copies the detail metadata URL exactly: %s',
    async (canonical) => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
      // A changed public base/path can make the detail read newer than inventory.
      setup([{ ...saml, samlMetadataUrl: 'https://old.example.test/metadata' }], {
        ...detail,
        samlMetadataUrl: canonical,
      });
      render(() => <SSOProvidersPanel />);
      const dialog = await edit();
      const copy = dialog.getByRole('button', { name: 'Copy SP metadata URL' });
      expect(copy).toHaveTextContent(canonical);
      expect(dialog.queryByText('https://old.example.test/metadata')).toBeNull();
      fireEvent.click(copy);
      await waitFor(() => expect(writeText).toHaveBeenCalledExactlyOnceWith(canonical));
      expect(await dialog.findByRole('status')).toHaveTextContent(
        'Metadata URL has been copied to the clipboard.',
      );
      expect(notices.success).not.toHaveBeenCalled();
      expect(dialog.getByRole('button', { name: 'Save Changes' })).toBeEnabled();
      fireEvent.click(dialog.getByRole('button', { name: 'Cancel' }));
      readOnlyRequests();
    },
  );

  it.each([undefined, ''])(
    'does not revive a metadata URL omitted from current details: %s',
    async (missing) => {
      setup([saml], { ...detail, samlMetadataUrl: missing, samlAcsUrl: missing });
      render(() => <SSOProvidersPanel />);
      const dialog = await edit();
      expect(dialog.queryByRole('button', { name: 'Copy SP metadata URL' })).toBeNull();
      expect(dialog.queryByText(metadata)).toBeNull();
      expect(dialog.getByText(/No SP Metadata URL available/)).toHaveTextContent(
        'Check Settings > System public URL and saved provider settings.',
      );
      expect(dialog.getByRole('textbox', { name: 'Provider Name' })).toHaveValue('Operations');
      expect(dialog.getByRole('textbox', { name: 'IdP SSO URL' })).toHaveValue(
        'https://idp.example.test/sso',
      );
      fireEvent.click(
        dialog.getByRole('button', { name: 'Show access restrictions & role mapping' }),
      );
      expect(dialog.getByRole('textbox', { name: 'Allowed Groups' })).toHaveValue('Operators');
      expect(dialog.getByRole('textbox', { name: 'Group Role Mappings' })).toHaveValue(
        'Operators=viewer',
      );
      expect(dialog.getByRole('button', { name: 'Save Changes' })).toBeEnabled();
      fireEvent.click(dialog.getByRole('button', { name: 'Cancel' }));
      readOnlyRequests();
    },
  );

  it('explains when a new SAML provider gets its URL instead of advertising a guessed placeholder', async () => {
    setup([]);
    render(() => <SSOProvidersPanel />);
    const add = await screen.findByRole('button', { name: 'Add SAML' });
    await waitFor(() => expect(add).toBeEnabled());
    fireEvent.click(add);
    const dialog = within(screen.getByRole('dialog', { name: 'Add SAML provider' }));
    expect(
      dialog.getByText(
        'The SP Metadata URL is generated when the provider is saved. Save this provider, then copy it from the provider card.',
      ),
    ).toBeVisible();
    expect(dialog.queryByText(/\/api\/saml\//)).toBeNull();
    expect(dialog.queryByRole('button', { name: 'Copy SP metadata URL' })).toBeNull();
    expect(dialog.getByRole('button', { name: 'Create Provider' })).toBeEnabled();
    fireEvent.click(dialog.getByRole('button', { name: 'Cancel' }));
    readOnlyRequests();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('keeps canonical metadata available when editing a disabled saved provider', async () => {
    setup([{ ...saml, enabled: false }], { ...detail, enabled: false });
    render(() => <SSOProvidersPanel />);
    const dialog = await edit();
    expect(dialog.getByRole('button', { name: 'Copy SP metadata URL' })).toHaveTextContent(
      metadata,
    );
    readOnlyRequests();
  });

  it('carries the OIDC registration step from the draft to the saved callback control', async () => {
    const canonical = 'https://pulse.example.test/edge/callback?tenant=ops%2fwest';
    const stale = 'https://different.example.test/callback';
    const withoutCallback = {
      id: 'legacy',
      name: 'Without callback',
      type: 'oidc',
      enabled: false,
      priority: 0,
    };
    const savedProvider = {
      ...withoutCallback,
      id: 'new-provider',
      name: 'Fleet login',
      oidcCallbackUrl: canonical,
    };
    const writeText = vi.fn().mockResolvedValue(undefined);
    const clipboard = Object.getOwnPropertyDescriptor(navigator, 'clipboard');
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    let created = false;
    fetchMock.mockImplementation(async (url: string, options?: RequestInit) => {
      if (url !== '/api/security/sso/providers') throw new Error('Unexpected request');
      if (options?.method === 'POST') {
        created = true;
        return json({ ...savedProvider, oidcCallbackUrl: stale });
      }
      return json({ providers: created ? [savedProvider, withoutCallback] : [withoutCallback] });
    });
    try {
      render(() => <SSOProvidersPanel />);
      const add = await screen.findByRole('button', { name: 'Add OIDC' });
      await waitFor(() => expect(add).toBeEnabled());
      const unavailable = screen.getByText(/No Callback \/ Redirect URL available/);
      expect(unavailable).toBeVisible();
      expect(screen.queryByText(/Register this URL as a redirect URI/)).toBeNull();
      fireEvent.click(add);
      const draft = within(screen.getByRole('dialog', { name: 'Add OIDC provider' }));
      expect(draft.getByText(/Create the provider in Pulse/)).toHaveTextContent(
        'Then copy its Callback / Redirect URL from the saved provider card and register it with your identity provider.',
      );
      expect(draft.queryByRole('button', { name: 'Copy callback / redirect URL' })).toBeNull();
      fireEvent.input(draft.getByLabelText('Provider Name'), { target: { value: 'Fleet login' } });
      fireEvent.input(draft.getByLabelText('Issuer URL'), {
        target: { value: 'https://identity.example.test/fleet' },
      });
      fireEvent.input(draft.getByLabelText('Client ID'), { target: { value: 'synthetic-fleet' } });
      fireEvent.click(draft.getByRole('button', { name: 'Create Provider' }));

      const copy = await screen.findByRole('button', { name: 'Copy callback / redirect URL' });
      expect(copy).toHaveTextContent(canonical);
      expect(copy).toHaveAccessibleDescription(
        'Register this URL as a redirect URI in your identity provider before testing sign-in.',
      );
      expect(screen.queryByText(stale)).toBeNull();
      expect(screen.getByText(/No Callback \/ Redirect URL available/)).toBeVisible();
      expect(screen.getAllByText(/Register this URL as a redirect URI/)).toHaveLength(1);
      fireEvent.click(copy);
      await waitFor(() => expect(writeText).toHaveBeenCalledExactlyOnceWith(canonical));
      expect(fetchMock.mock.calls.map(([, options]) => options?.method ?? 'GET')).toEqual([
        'GET',
        'POST',
        'GET',
      ]);
    } finally {
      if (clipboard) Object.defineProperty(navigator, 'clipboard', clipboard);
      else Reflect.deleteProperty(navigator, 'clipboard');
    }
  });

  it.each([undefined, 'https://new.example.test/base/callback?tenant=ops%2fwest'])(
    'uses the current detail callback for OIDC too, including withdrawal: %s',
    async (canonical) => {
      const oidc = {
        ...saml,
        type: 'oidc',
        oidcCallbackUrl: 'https://old.example.test/callback',
      };
      setup([oidc], {
        ...oidc,
        oidcCallbackUrl: canonical,
        oidc: { issuerUrl: 'https://idp.example.test', clientId: 'pulse' },
      });
      render(() => <SSOProvidersPanel />);
      const dialog = await edit('OIDC');
      expect(dialog.queryByText('https://old.example.test/callback')).toBeNull();
      if (canonical) {
        expect(
          dialog.getByRole('button', { name: 'Copy callback / redirect URL' }),
        ).toHaveTextContent(canonical);
      } else {
        expect(dialog.queryByRole('button', { name: 'Copy callback / redirect URL' })).toBeNull();
        expect(dialog.getByText(/No Callback/)).toBeVisible();
      }
      readOnlyRequests();
    },
  );
});
