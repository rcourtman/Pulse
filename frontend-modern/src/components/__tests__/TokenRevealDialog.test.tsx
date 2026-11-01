import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, fireEvent, waitFor } from '@solidjs/testing-library';
import { TokenRevealDialog } from '@/components/TokenRevealDialog';
import { copyToClipboard } from '@/utils/clipboard';
import { showSuccess, showError } from '@/utils/toast';
import { showTokenReveal, dismissTokenReveal } from '@/stores/tokenReveal';

vi.mock('@/utils/clipboard', () => ({ copyToClipboard: vi.fn() }));
vi.mock('@/utils/toast', () => ({ showSuccess: vi.fn(), showError: vi.fn() }));

function deferredCopy() {
  let settle!: (success: boolean) => void;
  const promise = new Promise<boolean>((resolve) => {
    settle = resolve;
  });
  vi.mocked(copyToClipboard).mockReturnValueOnce(promise);
  return settle;
}

function reveal(token = 'synthetic-token-a') {
  showTokenReveal({ token, record: mockRecord, source: 'security' });
}

const mockRecord = {
  id: 'test-token',
  name: 'Integration token',
  prefix: 'abc123',
  suffix: '7890',
  createdAt: new Date().toISOString(),
};

describe('TokenRevealDialog', () => {
  afterEach(() => {
    dismissTokenReveal();
    cleanup();
    vi.resetAllMocks();
    vi.useRealTimers();
  });

  it('renders and dismisses the token dialog', async () => {
    render(() => <TokenRevealDialog />);
    expect(screen.queryByText(/API token ready/i)).toBeNull();

    showTokenReveal({
      token: 'abc123token7890',
      record: mockRecord,
      source: 'test',
    });

    expect(await screen.findByText(/API token ready/i)).toBeInTheDocument();
    expect(screen.getByText(/abc123token7890/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /close token dialog/i }));
    expect(screen.queryByText(/API token ready/i)).toBeNull();
  });

  it('supports dismissal via escape key', async () => {
    render(() => <TokenRevealDialog />);

    showTokenReveal({
      token: 'escape-test-token',
      record: mockRecord,
      source: 'test',
    });

    expect(await screen.findByText(/API token ready/i)).toBeInTheDocument();

    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByText(/API token ready/i)).toBeNull();
  });

  it('renders canonical source badges for known platform aliases', async () => {
    render(() => <TokenRevealDialog />);

    showTokenReveal({
      token: 'pbs-token',
      record: mockRecord,
      source: 'pbs',
    });

    const badge = await screen.findByText('PBS');
    expect(badge.className).toContain('bg-indigo-100');
  });

  it('replaces every displayed field when another reveal becomes current', async () => {
    render(() => <TokenRevealDialog />);
    reveal();
    expect(await screen.findByText('synthetic-token-a')).toBeInTheDocument();
    showTokenReveal({
      token: 'synthetic-token-b',
      record: {
        ...mockRecord,
        id: 'other-token',
        name: 'Replacement token',
        prefix: 'next',
        suffix: 'abcd',
      },
      source: 'pbs',
      note: 'Replacement-only guidance',
    });
    expect(screen.queryByText('synthetic-token-a')).not.toBeInTheDocument();
    expect(screen.queryByText('Integration token')).not.toBeInTheDocument();
    expect(screen.getByText('synthetic-token-b')).toBeInTheDocument();
    expect(screen.getByText('Replacement token')).toBeInTheDocument();
    expect(screen.getByText('next…abcd')).toBeInTheDocument();
    expect(screen.getByText('PBS')).toBeInTheDocument();
    expect(screen.getByText('Replacement-only guidance')).toBeInTheDocument();
    vi.mocked(copyToClipboard).mockResolvedValueOnce(true);
    fireEvent.click(screen.getByRole('button', { name: 'Copy token' }));
    expect(copyToClipboard).toHaveBeenCalledExactlyOnceWith('synthetic-token-b');
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Copied!' })).toBeInTheDocument(),
    );
  });

  it('keeps long source, record and guidance strings as wrapping literal text', async () => {
    render(() => <TokenRevealDialog />);
    showTokenReveal({
      token: 'synthetic-<script>not-executable</script>',
      record: { ...mockRecord, name: '<img src=x onerror=alert(1)>' + 'a'.repeat(160) },
      source: 'unfamiliar-source-' + 'b'.repeat(120),
      note: '<b>Literal operator guidance</b>',
    });
    expect(
      await screen.findByText('synthetic-<script>not-executable</script>'),
    ).toBeInTheDocument();
    expect(screen.getByText('<b>Literal operator guidance</b>')).toBeInTheDocument();
    expect(screen.getByText('<img src=x onerror=alert(1)>' + 'a'.repeat(160))).toBeInTheDocument();
    const badge = screen.getByText(/Unfamiliar Source/);
    expect(badge).not.toHaveClass('whitespace-nowrap');
    expect(badge).toHaveClass('whitespace-normal');
    expect(screen.getByRole('dialog').querySelector('script, img')).toBeNull();
  });

  it('copies the current value once and reports pending and settled outcomes', async () => {
    const settle = deferredCopy();
    render(() => <TokenRevealDialog />);
    reveal();
    fireEvent.click(await screen.findByRole('button', { name: 'Copy token' }));
    const pending = screen.getByRole('button', { name: 'Copying…' });
    expect(pending).toBeDisabled();
    fireEvent.click(pending);
    expect(copyToClipboard).toHaveBeenCalledTimes(1);
    expect(copyToClipboard).toHaveBeenCalledWith('synthetic-token-a');
    expect(showSuccess).not.toHaveBeenCalled();
    settle(true);
    await waitFor(() => expect(screen.getByRole('button', { name: 'Copied!' })).toBeEnabled());
    expect(showSuccess).toHaveBeenCalledExactlyOnceWith('Token copied to clipboard');
  });

  it('reports failure only for the current attempt and allows a deliberate retry', async () => {
    const settle = deferredCopy();
    render(() => <TokenRevealDialog />);
    reveal();
    fireEvent.click(await screen.findByRole('button', { name: 'Copy token' }));
    settle(false);
    await waitFor(() => expect(showError).toHaveBeenCalledExactlyOnceWith('Failed to copy token'));
    expect(screen.getByRole('button', { name: 'Copy token' })).toBeEnabled();
    vi.mocked(copyToClipboard).mockResolvedValueOnce(true);
    fireEvent.click(screen.getByRole('button', { name: 'Copy token' }));
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Copied!' })).toBeInTheDocument(),
    );
    expect(copyToClipboard).toHaveBeenCalledTimes(2);
  });

  it('withdraws an earlier success when a deliberate retry fails', async () => {
    vi.mocked(copyToClipboard).mockResolvedValueOnce(true);
    render(() => <TokenRevealDialog />);
    reveal();
    fireEvent.click(await screen.findByRole('button', { name: 'Copy token' }));
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Copied!' })).toBeInTheDocument(),
    );
    const settle = deferredCopy();
    fireEvent.click(screen.getByRole('button', { name: 'Copied!' }));
    expect(screen.getByRole('button', { name: 'Copying…' })).toBeDisabled();
    settle(false);
    await waitFor(() => expect(screen.getByRole('button', { name: 'Copy token' })).toBeEnabled());
    expect(showSuccess).toHaveBeenCalledTimes(1);
    expect(showError).toHaveBeenCalledExactlyOnceWith('Failed to copy token');
  });

  it.each([true, false])(
    'ignores a %s clipboard result after dismissal without reopening',
    async (success) => {
      const settle = deferredCopy();
      render(() => <TokenRevealDialog />);
      reveal();
      fireEvent.click(await screen.findByRole('button', { name: 'Copy token' }));
      fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }));
      settle(success);
      await Promise.resolve();
      await Promise.resolve();
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(showSuccess).not.toHaveBeenCalled();
      expect(showError).not.toHaveBeenCalled();
    },
  );

  it.each([true, false])(
    'ignores a %s clipboard result after another reveal replaces it',
    async (success) => {
      const settle = deferredCopy();
      render(() => <TokenRevealDialog />);
      reveal();
      fireEvent.click(await screen.findByRole('button', { name: 'Copy token' }));
      // Reusing the same token bytes still creates a different reveal lifetime.
      reveal();
      settle(success);
      await Promise.resolve();
      await Promise.resolve();
      expect(screen.getByRole('button', { name: 'Copy token' })).toBeEnabled();
      expect(screen.queryByRole('button', { name: 'Copied!' })).not.toBeInTheDocument();
      expect(showSuccess).not.toHaveBeenCalled();
      expect(showError).not.toHaveBeenCalled();
    },
  );

  it.each([true, false])(
    'ignores a %s clipboard result after dismissal and reopening',
    async (success) => {
      const settle = deferredCopy();
      render(() => <TokenRevealDialog />);
      reveal();
      fireEvent.click(await screen.findByRole('button', { name: 'Copy token' }));
      fireEvent.click(screen.getByRole('button', { name: 'Close token dialog' }));
      reveal('synthetic-token-b');
      settle(success);
      await Promise.resolve();
      await Promise.resolve();
      expect(screen.getByRole('button', { name: 'Copy token' })).toBeEnabled();
      expect(showSuccess).not.toHaveBeenCalled();
      expect(showError).not.toHaveBeenCalled();
    },
  );

  it.each([true, false])(
    'ignores a %s clipboard result after the consuming dialog unmounts',
    async (success) => {
      const settle = deferredCopy();
      const { unmount } = render(() => <TokenRevealDialog />);
      reveal();
      fireEvent.click(await screen.findByRole('button', { name: 'Copy token' }));
      unmount();
      settle(success);
      await Promise.resolve();
      await Promise.resolve();
      expect(showSuccess).not.toHaveBeenCalled();
      expect(showError).not.toHaveBeenCalled();
    },
  );

  it('does not let a previous copied timer erase the current reveal feedback', async () => {
    vi.useFakeTimers();
    vi.mocked(copyToClipboard).mockResolvedValue(true);
    render(() => <TokenRevealDialog />);
    reveal();
    fireEvent.click(screen.getByRole('button', { name: 'Copy token' }));
    await Promise.resolve();
    expect(screen.getByRole('button', { name: 'Copied!' })).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(1500);
    reveal('synthetic-token-b');
    fireEvent.click(screen.getByRole('button', { name: 'Copy token' }));
    await Promise.resolve();
    await vi.advanceTimersByTimeAsync(600);
    expect(screen.getByRole('button', { name: 'Copied!' })).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(1400);
    expect(screen.getByRole('button', { name: 'Copy token' })).toBeInTheDocument();
  });
});
