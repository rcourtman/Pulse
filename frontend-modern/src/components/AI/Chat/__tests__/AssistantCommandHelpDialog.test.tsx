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

  it('finds local stream fixtures from command help search', () => {
    render(() => <AssistantCommandHelpDialog onClose={vi.fn()} onRunCommand={vi.fn()} />);

    fireEvent.input(screen.getByLabelText('Search Assistant commands'), {
      target: { value: 'provider-retry' },
    });

    expect(screen.getByRole('option', { name: /\/fixture/ })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: /\/fixture/ })).toHaveTextContent('/fixture');
  });

  it('finds queued follow-up management from command help search', () => {
    render(() => <AssistantCommandHelpDialog onClose={vi.fn()} onRunCommand={vi.fn()} />);

    fireEvent.input(screen.getByLabelText('Search Assistant commands'), {
      target: { value: 'queued' },
    });

    expect(screen.getByRole('option', { name: /\/queue/ })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: /\/queue/ })).toHaveTextContent(
      'Manage queued follow-ups',
    );
  });

  it('moves command selection with arrow keys before running', () => {
    const onRunCommand = vi.fn();
    render(() => <AssistantCommandHelpDialog onClose={vi.fn()} onRunCommand={onRunCommand} />);

    const sessionsCommand = screen.getByRole('option', { name: /\/sessions/ });
    expect(sessionsCommand).toHaveAttribute('aria-selected', 'false');

    fireEvent.keyDown(document, { key: 'ArrowDown' });
    expect(sessionsCommand).toHaveAttribute('aria-selected', 'true');

    fireEvent.keyDown(document, { key: 'Enter' });
    expect(onRunCommand).toHaveBeenCalledWith(
      expect.objectContaining({ action: 'sessions', name: 'sessions' }),
    );
  });

  it('groups unfiltered commands by purpose without making headers selectable', () => {
    render(() => <AssistantCommandHelpDialog onClose={vi.fn()} onRunCommand={vi.fn()} />);

    expect(screen.getByText('Session')).toBeInTheDocument();
    expect(screen.getByText('Model')).toBeInTheDocument();
    expect(screen.getByText('Transcript')).toBeInTheDocument();
    expect(screen.getByText('Help')).toBeInTheDocument();
    expect(screen.getAllByRole('option')).toHaveLength(14);
  });

  it('hides command group headers once help search is filtered', () => {
    render(() => <AssistantCommandHelpDialog onClose={vi.fn()} onRunCommand={vi.fn()} />);

    fireEvent.input(screen.getByLabelText('Search Assistant commands'), {
      target: { value: 'runtime' },
    });

    expect(screen.getByRole('option', { name: /\/status/ })).toBeInTheDocument();
    expect(screen.queryByText('Model')).not.toBeInTheDocument();
  });

  it('keeps disabled commands selectable so the caller can explain them', () => {
    const onRunCommand = vi.fn();
    render(() => (
      <AssistantCommandHelpDialog
        availability={{
          compact: {
            disabled: true,
            reason: 'Requires transcript content.',
          },
        }}
        onClose={vi.fn()}
        onRunCommand={onRunCommand}
      />
    ));

    fireEvent.input(screen.getByLabelText('Search Assistant commands'), {
      target: { value: 'compact' },
    });

    const compactCommand = screen.getByRole('option', { name: /\/compact/ });
    expect(compactCommand).toHaveAttribute('aria-disabled', 'true');
    expect(compactCommand).not.toBeDisabled();
    expect(screen.getByText('Requires transcript content.')).toBeInTheDocument();

    fireEvent.keyDown(document, { key: 'Enter' });
    expect(onRunCommand).toHaveBeenCalledWith(
      expect.objectContaining({ action: 'compact', name: 'compact' }),
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
