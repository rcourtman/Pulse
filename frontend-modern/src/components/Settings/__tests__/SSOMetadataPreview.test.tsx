import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { SSOProvidersPanel } from '../SSOProvidersPanel';

const fetchMock = vi.fn();
const loggerErrorMock = vi.fn();
const notificationErrorMock = vi.fn();
vi.mock('@/utils/logger', () => ({
  logger: { error: (...args: unknown[]) => loggerErrorMock(...args), warn: vi.fn() },
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: vi.fn(),
    error: (...args: unknown[]) => notificationErrorMock(...args),
  },
}));

const canary = 'SYNTHETIC_PRIVATE_METADATA_DETAIL';
const fallback =
  'Unable to preview SAML metadata. Check the connection and metadata URL, then try again. Your draft has not been saved.';
const fetchGuidance =
  'Unable to fetch SAML metadata. Check the URL, provider availability and TLS trust on the Pulse server, then try again. Your draft has not been saved.';
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const envelope = (code: unknown) =>
  json(
    { error: canary, code, details: { private: canary }, status_code: 400, timestamp: 1793078652 },
    400,
  );
const preview = {
  xml: '<EntityDescriptor entityID="https://idp.example.test/entity"/>',
  parsed: { entityId: 'https://idp.example.test/entity', ssoUrl: 'https://idp.example.test/sso' },
};
const cases = [
  {
    name: 'structured fetch error',
    response: () => envelope('fetch_error'),
    expected: fetchGuidance,
  },
  {
    name: 'structured validation error',
    response: () => envelope('validation_error'),
    expected:
      'Unable to preview SAML metadata. Check the metadata URL and try again. Your draft has not been saved.',
  },
  {
    name: 'structured parse error',
    response: () => envelope('parse_error'),
    expected:
      'Unable to parse SAML metadata. Check that the URL returns a SAML metadata document. Your draft has not been saved.',
  },
  {
    name: 'generic private preview error',
    response: () => envelope('preview_error'),
    expected: fallback,
  },
  { name: 'unknown failure code', response: () => envelope(canary), expected: fallback },
  {
    name: 'wrong-type failure code',
    response: () => envelope({ private: canary }),
    expected: fallback,
  },
  {
    name: 'legacy gateway page',
    response: () => new Response(`<html><pre>${canary}</pre></html>`, { status: 502 }),
    expected: fallback,
  },
  {
    name: 'legacy plain text',
    response: () => new Response(canary, { status: 400 }),
    expected: fallback,
  },
  {
    name: 'malformed JSON',
    response: () => new Response('{"error":', { status: 500 }),
    expected: fallback,
  },
  {
    name: 'oversized error',
    response: () => json({ error: canary.repeat(100) }, 400),
    expected: fallback,
  },
  {
    name: 'lost response',
    response: () => {
      throw new Error(canary);
    },
    expected: fallback,
  },
  {
    name: 'CSRF refusal without hidden retry',
    response: () => json({ error: canary }, 403),
    expected:
      'Metadata preview was refused. Check your administrator session and permissions, then try again. Your draft has not been saved.',
  },
  {
    name: 'rate limit without hidden retry',
    response: () =>
      new Response(JSON.stringify({ code: 'rate_limited', error: canary }), {
        status: 429,
        headers: { 'Retry-After': '0.001' },
      }),
    expected:
      'Too many metadata previews. Wait a moment and try again. Your draft has not been saved.',
  },
];

describe.each([false, true])('SAML metadata preview recovery (editing=%s)', (editing) => {
  const provider = {
    id: 'corp-saml',
    name: 'Corporate SAML',
    type: 'saml',
    enabled: true,
    priority: 0,
    allowedGroups: ['Server Access'],
    groupRoleMappings: { 'Server Access': 'viewer' },
    saml: { idpMetadataUrl: 'https://original.example.test/metadata' },
  };
  beforeEach(() => {
    vi.resetAllMocks();
    vi.stubGlobal('fetch', fetchMock);
    document.cookie = 'pulse_csrf=synthetic-csrf';
  });
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it.each(cases)(
    '$name preserves drafts and allows an explicit retry',
    async ({ response, expected }) => {
      let fail = true;
      fetchMock.mockImplementation(async (url: string) => {
        if (url.endsWith('/metadata/preview')) return fail ? response() : json(preview);
        if (url === '/api/security/status')
          return json({ publicUrl: 'https://pulse.example.test' });
        if (url.endsWith('/corp-saml')) return json(provider);
        return json({ providers: editing ? [provider] : [] });
      });
      render(() => <SSOProvidersPanel />);
      fireEvent.click(
        await screen.findByRole('button', { name: editing ? 'Edit provider' : 'Add SAML' }),
      );
      const editor = await screen.findByRole('dialog');
      const dialog = within(editor);
      const url = dialog.getByLabelText('IdP Metadata URL') as HTMLInputElement;
      const cert = dialog.getByLabelText('IdP Certificate (PEM)') as HTMLTextAreaElement;
      fireEvent.input(url, { target: { value: 'https://idp.example.test/metadata' } });
      fireEvent.input(cert, { target: { value: 'synthetic-local-certificate-draft' } });
      fireEvent.click(dialog.getByRole('button', { name: 'Preview' }));
      const alert = await dialog.findByRole('alert');
      expect(alert.textContent).toBe(expected);
      expect(alert.textContent).not.toContain(canary);
      expect(url.value).toBe('https://idp.example.test/metadata');
      expect(cert.value).toBe('synthetic-local-certificate-draft');
      expect(dialog.getByRole('button', { name: 'Preview' })).toBeEnabled();
      expect(dialog.getByRole('button', { name: 'Cancel' })).toBeEnabled();
      expect(notificationErrorMock).not.toHaveBeenCalled();
      expect(JSON.stringify(loggerErrorMock.mock.calls)).not.toContain(canary);
      expect(
        screen.queryByRole('dialog', { name: 'IdP metadata preview' }),
      ).not.toBeInTheDocument();

      const posts = () => fetchMock.mock.calls.filter(([, options]) => options?.method === 'POST');
      expect(posts()).toHaveLength(1);
      expect(JSON.parse(posts()[0][1].body)).toEqual({ type: 'saml', metadataUrl: url.value });

      fail = false;
      fireEvent.click(dialog.getByRole('button', { name: 'Preview' }));
      const result = await screen.findByRole('dialog', { name: 'IdP metadata preview' });
      expect(result).toHaveTextContent(preview.parsed.entityId);
      expect(result).toHaveTextContent(preview.parsed.ssoUrl);
      expect(posts()).toHaveLength(2);
      expect(posts()[1][1].body).toBe(posts()[0][1].body);
      expect(editor.querySelector('[role="alert"]')).toBeNull();
      fireEvent.click(within(result).getByRole('button', { name: 'Close metadata preview' }));
      expect(url.value).toBe('https://idp.example.test/metadata');
      expect(cert.value).toBe('synthetic-local-certificate-draft');
      expect(
        fetchMock.mock.calls.every(
          ([path, options]) => !options?.method || path.endsWith('/metadata/preview'),
        ),
      ).toBe(true);
      fireEvent.click(dialog.getByRole('button', { name: 'Cancel' }));
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    },
  );

  it('returns to Preview even when its pending disabled state loses focus', async () => {
    let finishPreview!: (response: Response) => void;
    fetchMock.mockImplementation(async (url: string) => {
      if (url.endsWith('/metadata/preview'))
        return new Promise<Response>((resolve) => {
          finishPreview = resolve;
        });
      if (url.endsWith('/corp-saml')) return json(provider);
      return json({ providers: editing ? [provider] : [] });
    });
    render(() => <SSOProvidersPanel />);
    fireEvent.click(
      await screen.findByRole('button', { name: editing ? 'Edit provider' : 'Add SAML' }),
    );
    const editor = within(await screen.findByRole('dialog'));
    fireEvent.input(editor.getByLabelText('IdP Metadata URL'), {
      target: { value: 'https://idp.example.test/metadata' },
    });
    const opener = editor.getByRole('button', { name: 'Preview' });
    opener.focus();
    fireEvent.click(opener);
    await waitFor(() => expect(finishPreview).toBeTypeOf('function'));
    expect(opener).toBeDisabled();
    // Native browsers can blur a button when it becomes disabled during fetch.
    // Do not rely on that transient focus as the nested Dialog's return target.
    (editor.getByLabelText('Provider Name') as HTMLInputElement).focus();
    finishPreview(json(preview));
    const result = await screen.findByRole('dialog', { name: 'IdP metadata preview' });
    fireEvent.click(within(result).getByRole('button', { name: 'Close metadata preview' }));
    await waitFor(() => expect(opener).toHaveFocus());
    expect(opener).toBeEnabled();
  });

  it('does not dispatch a blank URL or interpret preview as saving', async () => {
    fetchMock.mockImplementation(async (url: string) =>
      url.endsWith('/corp-saml') ? json(provider) : json({ providers: editing ? [provider] : [] }),
    );
    render(() => <SSOProvidersPanel />);
    fireEvent.click(
      await screen.findByRole('button', { name: editing ? 'Edit provider' : 'Add SAML' }),
    );
    const dialog = within(await screen.findByRole('dialog'));
    fireEvent.input(dialog.getByLabelText('IdP Metadata URL'), { target: { value: '  ' } });
    expect(dialog.getByRole('button', { name: 'Preview' })).toBeDisabled();
    expect(dialog.queryByRole('alert')).toBeNull();
    expect(fetchMock.mock.calls.every(([, options]) => !options?.method)).toBe(true);
  });
});
