import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { AssistantCommandHelpDialog } from '../AssistantCommandHelpDialog';

afterEach(cleanup);

describe('AssistantCommandHelpDialog', () => {
  it('consumes Escape as a local dialog close command', () => {
    const onClose = vi.fn();
    render(() => <AssistantCommandHelpDialog onClose={onClose} onRunCommand={vi.fn()} />);
    expect(screen.getByRole('dialog', { name: 'Assistant commands' })).toBeInTheDocument();

    const laterDocumentHandler = vi.fn();
    document.addEventListener('keydown', laterDocumentHandler);
    const escapeEvent = new KeyboardEvent('keydown', {
      bubbles: true,
      cancelable: true,
      key: 'Escape',
    });
    document.dispatchEvent(escapeEvent);

    document.removeEventListener('keydown', laterDocumentHandler);
    expect(escapeEvent.defaultPrevented).toBe(true);
    expect(laterDocumentHandler).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledOnce();
  });
});
