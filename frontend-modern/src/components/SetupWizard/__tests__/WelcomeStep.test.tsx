import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { WelcomeStep } from '../steps/WelcomeStep';

const apiFetchMock = vi.fn();
const apiFetchJSONMock = vi.fn();
const copyToClipboardMock = vi.fn();
const showErrorMock = vi.fn();
const showSuccessMock = vi.fn();

vi.mock('@/utils/apiClient', () => ({
  apiFetch: (...args: unknown[]) => apiFetchMock(...args),
  apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
}));

vi.mock('@/utils/clipboard', () => ({
  copyToClipboard: (...args: unknown[]) => copyToClipboardMock(...args),
}));

vi.mock('@/utils/toast', () => ({
  showError: (...args: unknown[]) => showErrorMock(...args),
  showSuccess: (...args: unknown[]) => showSuccessMock(...args),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  },
}));

describe('WelcomeStep', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    copyToClipboardMock.mockResolvedValue(true);
    apiFetchJSONMock.mockResolvedValue({
      bootstrapTokenPath: '/etc/pulse/.bootstrap_token',
      isDocker: false,
      inContainer: false,
      lxcCtid: '',
      dockerContainerName: '',
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    cleanup();
  });

  it('frames first-run setup as a clear three-step journey', async () => {
    render(() => (
      <WelcomeStep
        onNext={vi.fn()}
        bootstrapToken=""
        setBootstrapToken={vi.fn()}
        isUnlocked={false}
        setIsUnlocked={vi.fn()}
      />
    ));

    expect(apiFetchJSONMock).not.toHaveBeenCalled();

    expect(screen.getByText('Unlock this Pulse server')).toBeInTheDocument();
    expect(screen.getByText('Create the admin account')).toBeInTheDocument();
    expect(screen.getByText('Choose the first source')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Connect a platform API, install Pulse Agent, or use both for full coverage.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('Usage telemetry is enabled by default')).toBeInTheDocument();
    expect(
      screen.getByText(/To disable it before any ping, set PULSE_TELEMETRY=false/),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Full details' })).toHaveAttribute(
      'href',
      '/docs/PRIVACY.md',
    );
    expect(screen.getByText('What this token does')).toBeInTheDocument();
    expect(
      screen.getByText(
        'This one-time bootstrap token only unlocks first-run setup on this Pulse server. Run the matching command above and paste the token string it prints. It is not your admin password or long-lived API token.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Verify bootstrap token →' })).toBeInTheDocument();
  });

  it('offers generic commands without requesting deployment details from the server', () => {
    apiFetchJSONMock.mockResolvedValue({
      bootstrapTokenPath: '/srv/pulse/bootstrap.token',
      isDocker: true,
      inContainer: false,
      lxcCtid: '',
      dockerContainerName: 'pulse-main',
    });

    render(() => (
      <WelcomeStep
        onNext={vi.fn()}
        bootstrapToken=""
        setBootstrapToken={vi.fn()}
        isUnlocked={false}
        setIsUnlocked={vi.fn()}
      />
    ));

    expect(
      screen.getByText(
        'Run the command that matches how Pulse is installed. The server does not reveal deployment details before setup is unlocked.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('sudo pulse bootstrap-token')).toBeInTheDocument();
    expect(
      screen.getByText('docker exec <pulse-container> /app/pulse bootstrap-token'),
    ).toBeInTheDocument();
    expect(screen.getByText('pct exec <ctid> -- pulse bootstrap-token')).toBeInTheDocument();
    expect(screen.queryByText(/pulse-main/)).not.toBeInTheDocument();
    expect(apiFetchJSONMock).not.toHaveBeenCalled();
  });

  it('validates raw bootstrap tokens with an unauthenticated JSON request', async () => {
    const rawToken = '0123456789abcdef0123456789abcdef0123456789abcdef';
    const onNext = vi.fn();
    const setIsUnlocked = vi.fn();

    apiFetchMock.mockResolvedValue({ ok: true });

    render(() => (
      <WelcomeStep
        onNext={onNext}
        bootstrapToken={`  ${rawToken}  `}
        setBootstrapToken={vi.fn()}
        isUnlocked={false}
        setIsUnlocked={setIsUnlocked}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Verify bootstrap token →' }));

    await waitFor(() => {
      expect(apiFetchMock).toHaveBeenCalledWith('/api/security/validate-bootstrap-token', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        skipAuth: true,
        skipOrgContext: true,
        body: JSON.stringify({ token: rawToken }),
      });
    });
    expect(setIsUnlocked).toHaveBeenCalledWith(true);
    expect(onNext).toHaveBeenCalledTimes(1);
    expect(showErrorMock).not.toHaveBeenCalled();
  });

  it('waits for an explicit verification action after a token is pasted', async () => {
    vi.useFakeTimers();
    const rawToken = '0123456789abcdef0123456789abcdef0123456789abcdef';
    const onNext = vi.fn();

    const Harness = () => {
      const [bootstrapToken, setBootstrapToken] = createSignal('');
      return (
        <WelcomeStep
          onNext={onNext}
          bootstrapToken={bootstrapToken()}
          setBootstrapToken={setBootstrapToken}
          isUnlocked={false}
          setIsUnlocked={vi.fn()}
        />
      );
    };

    render(() => <Harness />);
    fireEvent.input(screen.getByPlaceholderText('Paste your bootstrap token'), {
      target: { value: rawToken },
    });

    await vi.advanceTimersByTimeAsync(500);

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onNext).not.toHaveBeenCalled();
    vi.useRealTimers();
  });

  it('keeps bootstrap validation single-flight across repeated submit actions', async () => {
    const rawToken = '0123456789abcdef0123456789abcdef0123456789abcdef';
    const onNext = vi.fn();
    const setIsUnlocked = vi.fn();
    let resolveValidation: ((value: { ok: boolean }) => void) | undefined;
    apiFetchMock.mockReturnValue(
      new Promise<{ ok: boolean }>((resolve) => {
        resolveValidation = resolve;
      }),
    );

    render(() => (
      <WelcomeStep
        onNext={onNext}
        bootstrapToken={rawToken}
        setBootstrapToken={vi.fn()}
        isUnlocked={false}
        setIsUnlocked={setIsUnlocked}
      />
    ));

    const verifyButton = screen.getByRole('button', { name: 'Verify bootstrap token →' });
    const tokenInput = screen.getByDisplayValue(rawToken);
    fireEvent.click(verifyButton);
    fireEvent.keyDown(tokenInput, { key: 'Enter' });

    expect(apiFetchMock).toHaveBeenCalledTimes(1);

    resolveValidation?.({ ok: true });
    await waitFor(() => expect(onNext).toHaveBeenCalledTimes(1));
    expect(setIsUnlocked).toHaveBeenCalledTimes(1);
  });

  it('blocks encrypted bootstrap snapshot pastes with a specific error', async () => {
    const onNext = vi.fn();

    render(() => (
      <WelcomeStep
        onNext={onNext}
        bootstrapToken='{"version":2,"token_ciphertext":"cipher","token_hash":"hash"}'
        setBootstrapToken={vi.fn()}
        isUnlocked={false}
        setIsUnlocked={vi.fn()}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Verify bootstrap token →' }));

    expect(showErrorMock).toHaveBeenCalledWith(
      'That looks like the encrypted .bootstrap_token file contents, not the raw setup token. Run the matching command above and paste the token string it prints.',
    );
    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onNext).not.toHaveBeenCalled();
  });
});
