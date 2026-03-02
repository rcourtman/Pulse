import { describe, expect, it, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@solidjs/testing-library';
import { ErrorDetail } from '../ErrorDetail';

afterEach(cleanup);

describe('ErrorDetail', () => {
  /* ── Rendering basics ──────────────────────────────────────────── */

  it('renders nothing when message is undefined', () => {
    const { container } = render(() => <ErrorDetail message={undefined} />);
    expect(container.innerHTML).toBe('');
  });

  it('renders nothing when message is empty string', () => {
    // Show when={props.message} is falsy for empty string
    const { container } = render(() => <ErrorDetail message="" />);
    expect(container.innerHTML).toBe('');
  });

  it('renders a short message inline without truncation', () => {
    render(() => <ErrorDetail message="Something went wrong" />);
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    // No "more" button for short messages
    expect(screen.queryByText('more')).not.toBeInTheDocument();
  });

  /* ── Long messages (> 60 chars) ────────────────────────────────── */

  const longMessage =
    'This is a very long error message that exceeds the sixty character inline limit for display purposes';

  it('truncates long messages and shows "more" button', () => {
    render(() => <ErrorDetail message={longMessage} />);
    // Should show truncated text (first 60 chars + ellipsis)
    expect(screen.getByText(/This is a very long error message that exceeds the sixty ch/)).toBeInTheDocument();
    expect(screen.getByText('more')).toBeInTheDocument();
    // Full message should NOT be visible initially
    expect(screen.queryByText(longMessage)).not.toBeInTheDocument();
  });

  it('expands to full message when "more" is clicked', async () => {
    render(() => <ErrorDetail message={longMessage} />);
    await fireEvent.click(screen.getByText('more'));
    // Full message should now be visible
    expect(screen.getByText(longMessage)).toBeInTheDocument();
    // "less" button should appear
    expect(screen.getByText('less')).toBeInTheDocument();
    // "more" button should be gone
    expect(screen.queryByText('more')).not.toBeInTheDocument();
  });

  it('collapses back when "less" is clicked', async () => {
    render(() => <ErrorDetail message={longMessage} />);
    await fireEvent.click(screen.getByText('more'));
    expect(screen.getByText('less')).toBeInTheDocument();

    await fireEvent.click(screen.getByText('less'));
    // Should be truncated again
    expect(screen.getByText('more')).toBeInTheDocument();
    expect(screen.queryByText(longMessage)).not.toBeInTheDocument();
  });

  it('stops event propagation on "more" click', async () => {
    const parentHandler = vi.fn();
    render(() => (
      <div onClick={parentHandler}>
        <ErrorDetail message={longMessage} />
      </div>
    ));
    await fireEvent.click(screen.getByText('more'));
    expect(parentHandler).not.toHaveBeenCalled();
  });

  it('stops event propagation on "less" click', async () => {
    const parentHandler = vi.fn();
    render(() => (
      <div onClick={parentHandler}>
        <ErrorDetail message={longMessage} />
      </div>
    ));
    // Expand first
    await fireEvent.click(screen.getByText('more'));
    // Now collapse
    await fireEvent.click(screen.getByText('less'));
    expect(parentHandler).not.toHaveBeenCalled();
  });

  /* ── Boundary: exactly 60 chars ─────────────────────────────────── */

  it('does not truncate a message of exactly 60 characters', () => {
    const exact60 = 'A'.repeat(60);
    render(() => <ErrorDetail message={exact60} />);
    expect(screen.getByText(exact60)).toBeInTheDocument();
    expect(screen.queryByText('more')).not.toBeInTheDocument();
  });

  it('truncates a message of 61 characters', () => {
    const msg61 = 'B'.repeat(61);
    render(() => <ErrorDetail message={msg61} />);
    expect(screen.getByText('more')).toBeInTheDocument();
    // Verify truncated portion is shown (first 60 chars + ellipsis)
    expect(screen.getByText(/B{60}…/)).toBeInTheDocument();
    // Full message should not be visible
    expect(screen.queryByText(msg61)).not.toBeInTheDocument();
  });

  /* ── Friendly error hints ──────────────────────────────────────── */

  it('shows hint for "connection refused" errors', () => {
    render(() => <ErrorDetail message="dial tcp: connection refused" />);
    expect(screen.getByText(/SSH connection.*Verify SSH is running/)).toBeInTheDocument();
  });

  it('shows hint for "permission denied" errors', () => {
    render(() => <ErrorDetail message="ssh: permission denied (publickey)" />);
    expect(screen.getByText(/SSH authentication failed/)).toBeInTheDocument();
  });

  it('shows hint for "timed out" errors', () => {
    render(() => <ErrorDetail message="dial tcp: i/o timed out" />);
    expect(screen.getByText(/connection timed out.*network connectivity/)).toBeInTheDocument();
  });

  it('shows hint for "timeout" errors', () => {
    render(() => <ErrorDetail message="context deadline exceeded (timeout)" />);
    expect(screen.getByText(/connection timed out.*network connectivity/)).toBeInTheDocument();
  });

  it('shows hint for "no route to host" errors', () => {
    render(() => <ErrorDetail message="connect: no route to host" />);
    expect(screen.getByText(/No network route.*Verify the IP address/)).toBeInTheDocument();
  });

  it('shows hint for "host key verification" errors', () => {
    render(() => <ErrorDetail message="Host key verification failed" />);
    expect(screen.getByText(/host key verification failed.*known_hosts/)).toBeInTheDocument();
  });

  it('does not show a hint for unknown error messages', () => {
    render(() => <ErrorDetail message="something unexpected" />);
    expect(screen.getByText('something unexpected')).toBeInTheDocument();
    // No hint paragraph — only the error text should be present
    const container = screen.getByText('something unexpected').closest('div');
    expect(container?.querySelectorAll('p').length).toBe(0);
  });

  it('does not show hint while long message is collapsed, shows it when expanded', async () => {
    const longConnectionError =
      'Failed to connect to node-01.example.com:22 — dial tcp 10.0.0.1:22: connection refused after 3 retries';
    render(() => <ErrorDetail message={longConnectionError} />);

    // While collapsed, hint should NOT be visible
    expect(screen.queryByText(/SSH connection.*Verify SSH is running/)).not.toBeInTheDocument();

    // Expand
    await fireEvent.click(screen.getByText('more'));

    // After expanding, hint and full message should be visible
    expect(screen.getByText(/SSH connection.*Verify SSH is running/)).toBeInTheDocument();
    expect(screen.getByText(longConnectionError)).toBeInTheDocument();
  });

  it('hint matching is case-insensitive', () => {
    render(() => <ErrorDetail message="CONNECTION REFUSED on port 22" />);
    expect(screen.getByText(/SSH connection.*Verify SSH is running/)).toBeInTheDocument();
  });
});
