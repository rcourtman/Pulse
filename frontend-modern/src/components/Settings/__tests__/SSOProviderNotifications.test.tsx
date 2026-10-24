import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { SSOProvidersPanel } from '../SSOProvidersPanel';
import { ToastContainer } from '@/components/Toast/Toast';

const provider = {
  id: 'corp-oidc',
  name: 'Corporate OIDC',
  type: 'oidc',
  enabled: true,
  priority: 0,
  oidc: { issuerUrl: 'https://idp.example.test', clientId: 'pulse' },
};

describe('SSO provider consuming notification layers', () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it('reveals a modal-only delete failure without losing the retained-success reopen behaviour', async () => {
    const fetchMock = vi.fn((url: string, options?: RequestInit) => {
      const method = options?.method ?? 'GET';
      let body: unknown;
      let status = 200;
      if (url === '/api/security/status') body = { publicUrl: 'https://pulse.example.test' };
      else if (method === 'DELETE') {
        body = { error: 'Synthetic provider deletion refused' };
        status = 409;
      } else if (url === '/api/security/sso/providers' && method === 'GET')
        body = { providers: [provider] };
      else body = provider;
      return Promise.resolve(
        new Response(JSON.stringify(body), {
          status,
          headers: { 'Content-Type': 'application/json' },
        }),
      );
    });
    vi.stubGlobal('fetch', fetchMock);
    document.cookie = 'pulse_csrf=synthetic-csrf';
    render(() => (
      <>
        <SSOProvidersPanel />
        <ToastContainer />
      </>
    ));
    fireEvent.click(await screen.findByRole('button', { name: 'Edit provider' }));
    let editor = await screen.findByRole('dialog', { name: 'Edit OIDC provider' });
    fireEvent.submit(within(editor).getByRole('button', { name: 'Save Changes' }).closest('form')!);
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    const notifications = screen.getByRole('region', { name: 'Notifications' });
    expect(notifications).toHaveTextContent('SSO provider has been updated.');

    fireEvent.click(await screen.findByRole('button', { name: 'Edit provider' }));
    editor = await screen.findByRole('dialog', { name: 'Edit OIDC provider' });
    expect(notifications).toHaveClass('z-50');
    expect(notifications.closest('[inert]')).not.toBeNull();
    fireEvent.click(within(editor).getByRole('button', { name: 'Cancel' }));

    fireEvent.click(screen.getByRole('button', { name: 'Delete provider' }));
    const confirmation = screen.getByRole('dialog', { name: 'Delete provider' });
    fireEvent.click(within(confirmation).getByRole('button', { name: 'Delete Provider' }));
    await waitFor(() =>
      expect(notifications).toHaveTextContent('Unable to remove the SSO provider.'),
    );
    expect(confirmation).toBeInTheDocument();
    expect(notifications).toHaveClass('z-9999');
    expect(notifications).not.toHaveClass('z-50');
    expect(notifications.closest('[inert]')).not.toBeNull();
    expect(fetchMock.mock.calls.filter(([, options]) => options?.method === 'DELETE')).toHaveLength(
      1,
    );
    fireEvent.click(within(confirmation).getByRole('button', { name: 'Cancel' }));
    expect(notifications.closest('[inert]')).toBeNull();
    expect(notifications).toHaveTextContent('Unable to remove the SSO provider.');
  });
});
