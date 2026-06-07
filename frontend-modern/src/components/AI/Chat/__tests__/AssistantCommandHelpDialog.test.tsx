import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { AssistantCommandHelpDialog } from '../AssistantCommandHelpDialog';

afterEach(cleanup);

describe('AssistantCommandHelpDialog', () => {
  it('focuses command search and filters commands through the shared slash matcher', async () => {
    render(() => <AssistantCommandHelpDialog onClose={vi.fn()} onRunCommand={vi.fn()} />);

    const search = screen.getByLabelText('Search Assistant commands');
    await waitFor(() => expect(document.activeElement).toBe(search));

    fireEvent.input(search, { target: { value: 'runtime' } });

    expect(screen.getByRole('listbox', { name: 'Assistant commands' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: /\/status/ })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: /\/models/ })).not.toBeInTheDocument();
  });

  it('runs the selected filtered command with Enter', () => {
    const onRunCommand = vi.fn();
    render(() => <AssistantCommandHelpDialog onClose={vi.fn()} onRunCommand={onRunCommand} />);

    fireEvent.input(screen.getByLabelText('Search Assistant commands'), {
      target: { value: 'runtime' },
    });
    fireEvent.keyDown(document, { key: 'Enter' });

    expect(onRunCommand).toHaveBeenCalledWith(
      expect.objectContaining({ action: 'status', name: 'status' }),
    );
  });

  it('moves command selection with arrow keys before running', () => {
    const onRunCommand = vi.fn();
    render(() => <AssistantCommandHelpDialog onClose={vi.fn()} onRunCommand={onRunCommand} />);

    const newCommand = screen.getByRole('option', { name: /\/new/ });
    expect(newCommand).toHaveAttribute('aria-selected', 'false');

    fireEvent.keyDown(document, { key: 'ArrowDown' });
    expect(newCommand).toHaveAttribute('aria-selected', 'true');

    fireEvent.keyDown(document, { key: 'Enter' });
    expect(onRunCommand).toHaveBeenCalledWith(
      expect.objectContaining({ action: 'new', name: 'new' }),
    );
  });

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
