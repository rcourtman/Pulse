import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Router, Route } from '@solidjs/router';

import HostedSignup from '@/pages/HostedSignup';

const signupMock = vi.fn();
const requestMagicLinkMock = vi.fn();

vi.mock('@/api/hostedSignup', () => ({
  HostedSignupAPI: {
    signup: (...args: unknown[]) => signupMock(...args),
    requestMagicLink: (...args: unknown[]) => requestMagicLinkMock(...args),
  },
}));

vi.mock('@/stores/license', () => ({
  getUpgradeActionUrlOrFallback: () => 'https://cloud.pulserelay.pro',
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    warn: vi.fn(),
  },
}));

describe('HostedSignup', () => {
  beforeEach(() => {
    signupMock.mockReset();
    requestMagicLinkMock.mockReset();
    signupMock.mockResolvedValue({
      ok: true,
      status: 201,
      data: {
        message: 'Check your email for a sign-in link.',
      },
    });
    requestMagicLinkMock.mockResolvedValue({
      ok: true,
      status: 200,
      data: {
        success: true,
        message: "If that email is registered, you'll receive a magic link shortly.",
      },
    });
    window.history.replaceState({}, '', '/cloud/signup?tier=power');
  });

  afterEach(() => {
    cleanup();
    window.history.replaceState({}, '', '/');
  });

  it('uses the selected tier from the URL in both copy and signup payload', async () => {
    render(() => (
      <Router>
        <Route path="/cloud/signup" component={HostedSignup} />
      </Router>
    ));

    expect(await screen.findByText('Create Power Hosted Workspace')).toBeInTheDocument();
    expect(screen.getByText(/30 agents included/i)).toBeInTheDocument();

    fireEvent.input(screen.getByLabelText('Work Email'), {
      target: { value: 'owner@example.com' },
    });
    fireEvent.input(screen.getByLabelText('Organization Name'), {
      target: { value: 'Pulse Labs' },
    });
    fireEvent.submit(screen.getByRole('button', { name: 'Create Hosted Workspace' }).closest('form')!);

    await waitFor(() => {
      expect(signupMock).toHaveBeenCalledWith({
        email: 'owner@example.com',
        org_name: 'Pulse Labs',
        tier: 'power',
      });
    });
  });
});
