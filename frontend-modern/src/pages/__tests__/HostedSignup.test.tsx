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

vi.mock('@/stores/licenseCommercial', () => ({
  getUpgradeActionDestination: () => ({ href: '/cloud', external: false }),
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
      status: 202,
      data: {
        message: "If that email can finish signup, you'll receive a Pulse Account sign-in link shortly.",
      },
    });
    requestMagicLinkMock.mockResolvedValue({
      ok: true,
      status: 200,
      data: {
        success: true,
        message: "If that email is registered, you'll receive a Pulse Account sign-in link shortly.",
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

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(
      screen.getByText('Start your 14-day Pulse Cloud trial and hosted workspace.'),
    ).toBeInTheDocument();
    expect(screen.getByText('Plan')).toBeInTheDocument();
    expect(screen.getByText('How it works')).toBeInTheDocument();
    expect(
      screen.getByText('Choose a Cloud plan and start the 14-day trial in secure checkout.'),
    ).toBeInTheDocument();
    expect(screen.getByText('Already signed up?')).toBeInTheDocument();
    expect(screen.getByText('Request a fresh Pulse Account sign-in link.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Email Pulse Account Link' })).toBeInTheDocument();

    fireEvent.input(screen.getByLabelText('Work Email'), {
      target: { value: 'owner@example.com' },
    });
    fireEvent.input(screen.getByLabelText('Organization Name'), {
      target: { value: 'Pulse Labs' },
    });
    fireEvent.submit(screen.getByRole('button', { name: 'Start Trial in Checkout' }).closest('form')!);

    await waitFor(() => {
      expect(signupMock).toHaveBeenCalledWith({
        email: 'owner@example.com',
        org_name: 'Pulse Labs',
        tier: 'power',
      });
    });
  });

  it('renders signup plan pricing from the shared cloud pricing contract', async () => {
    window.history.replaceState({}, '', '/cloud/signup?tier=starter');

    render(() => (
      <Router>
        <Route path="/cloud/signup" component={HostedSignup} />
      </Router>
    ));

    expect(await screen.findByText('Plan')).toBeInTheDocument();
    expect(screen.getByText('$19/month')).toBeInTheDocument();
    expect(screen.getByText('$29/month')).toBeInTheDocument();
    expect(screen.getByText('or $249/year (save 29%)')).toBeInTheDocument();
  });
});
