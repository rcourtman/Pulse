import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { CopyCommandBlock } from '../CopyCommandBlock';
import { copyToClipboard } from '@/utils/clipboard';

vi.mock('@/utils/clipboard', () => ({
  copyToClipboard: vi.fn(async () => true),
}));

describe('CopyCommandBlock', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('copies through the shared clipboard utility before notifying callers', async () => {
    const command = 'pulse-mcp --base-url http://localhost:7655';
    const onCopy = vi.fn();

    render(() => <CopyCommandBlock command={command} onCopy={onCopy} />);

    await fireEvent.click(screen.getByRole('button', { name: 'Copy to clipboard' }));

    await waitFor(() => expect(copyToClipboard).toHaveBeenCalledWith(command));
    expect(onCopy).toHaveBeenCalledWith(command);
  });

  it('does not report copied state when clipboard copy fails', async () => {
    vi.mocked(copyToClipboard).mockResolvedValueOnce(false);
    const onCopy = vi.fn();

    render(() => <CopyCommandBlock command="install command" onCopy={onCopy} />);

    await fireEvent.click(screen.getByRole('button', { name: 'Copy to clipboard' }));

    await waitFor(() => expect(copyToClipboard).toHaveBeenCalledWith('install command'));
    expect(onCopy).not.toHaveBeenCalled();
  });
});
