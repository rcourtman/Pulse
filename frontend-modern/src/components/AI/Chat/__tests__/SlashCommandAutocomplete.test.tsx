import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { filterAssistantSlashCommands } from '../assistantSlashCommands';
import { SlashCommandAutocomplete } from '../SlashCommandAutocomplete';

afterEach(cleanup);

describe('SlashCommandAutocomplete', () => {
  it('consumes local command navigation keys before later document handlers see them', () => {
    const onClose = vi.fn();
    const onSelect = vi.fn();
    render(() => (
      <SlashCommandAutocomplete
        query="mo"
        visible
        position={{ top: 58, left: 0 }}
        onClose={onClose}
        onSelect={onSelect}
      />
    ));
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
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('consumes selection keys while running the selected local command', () => {
    const onSelect = vi.fn();
    render(() => (
      <SlashCommandAutocomplete
        query="mo"
        visible
        position={{ top: 58, left: 0 }}
        onClose={vi.fn()}
        onSelect={onSelect}
      />
    ));
    const laterDocumentHandler = vi.fn();
    document.addEventListener('keydown', laterDocumentHandler);

    const enterEvent = new KeyboardEvent('keydown', {
      bubbles: true,
      cancelable: true,
      key: 'Enter',
    });
    document.dispatchEvent(enterEvent);

    document.removeEventListener('keydown', laterDocumentHandler);
    expect(enterEvent.defaultPrevented).toBe(true);
    expect(laterDocumentHandler).not.toHaveBeenCalled();
    expect(onSelect).toHaveBeenCalledWith(
      expect.objectContaining({
        action: 'models',
        name: 'models',
      }),
    );
  });

  it('wraps keyboard selection from the first command to the last command', () => {
    const onSelect = vi.fn();
    const lastCommand = filterAssistantSlashCommands('').at(-1);
    render(() => (
      <SlashCommandAutocomplete
        query=""
        visible
        position={{ top: 58, left: 0 }}
        onClose={vi.fn()}
        onSelect={onSelect}
      />
    ));

    document.dispatchEvent(
      new KeyboardEvent('keydown', {
        bubbles: true,
        cancelable: true,
        key: 'ArrowUp',
      }),
    );
    document.dispatchEvent(
      new KeyboardEvent('keydown', {
        bubbles: true,
        cancelable: true,
        key: 'Enter',
      }),
    );

    expect(onSelect).toHaveBeenCalledWith(
      expect.objectContaining({
        action: lastCommand?.action,
        name: lastCommand?.name,
      }),
    );
  });

  it('renders the full local command list with an icon for every command', () => {
    const { container } = render(() => (
      <SlashCommandAutocomplete
        query=""
        visible
        position={{ top: 58, left: 0 }}
        onClose={vi.fn()}
        onSelect={vi.fn()}
      />
    ));

    const activeCommands = filterAssistantSlashCommands('');
    const options = screen.getAllByRole('option');
    expect(options).toHaveLength(activeCommands.length);
    expect(options.map((option) => option.getAttribute('aria-label'))).toContain(
      'Run /compact: Summarize older turns and keep this session moving',
    );
    expect(container.querySelectorAll('[role="option"] svg')).toHaveLength(activeCommands.length);
    expect(screen.getByText('Session')).toBeInTheDocument();
    expect(screen.getByText('Model')).toBeInTheDocument();
    expect(screen.getByText('Transcript')).toBeInTheDocument();
    expect(screen.getByText('Developer')).toBeInTheDocument();
  });

  it('hides command group headers once the user filters', () => {
    render(() => (
      <SlashCommandAutocomplete
        query="runtime"
        visible
        position={{ top: 58, left: 0 }}
        onClose={vi.fn()}
        onSelect={vi.fn()}
      />
    ));

    expect(screen.getByRole('option', { name: /\/status/ })).toBeInTheDocument();
    expect(screen.queryByText('Model')).not.toBeInTheDocument();
  });

  it('surfaces dev stream fixtures through slash command search', () => {
    render(() => (
      <SlashCommandAutocomplete
        query="provider-retry"
        visible
        position={{ top: 58, left: 0 }}
        onClose={vi.fn()}
        onSelect={vi.fn()}
      />
    ));

    expect(
      screen.getByRole('option', {
        name: /Insert \/fixture: Run a local stream fixture by name/,
      }),
    ).toBeInTheDocument();
  });

  it('renders disabled matching commands with the unavailable reason', () => {
    render(() => (
      <SlashCommandAutocomplete
        availability={{
          compact: {
            disabled: true,
            reason: 'Requires transcript content.',
          },
        }}
        query="compact"
        visible
        position={{ top: 58, left: 0 }}
        onClose={vi.fn()}
        onSelect={vi.fn()}
      />
    ));

    expect(screen.getByRole('listbox', { name: 'Assistant commands' })).toBeInTheDocument();
    const option = screen.getByRole('option', {
      name: /Unavailable \/compact: Summarize older turns and keep this session moving\. Requires transcript content\./,
    });
    expect(option).toHaveAttribute('aria-disabled', 'true');
    expect(option).toHaveTextContent('Requires transcript content.');
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('consumes selection keys without selecting when no commands match', () => {
    const onSelect = vi.fn();
    render(() => (
      <SlashCommandAutocomplete
        query="not-a-command"
        visible
        position={{ top: 58, left: 0 }}
        onClose={vi.fn()}
        onSelect={onSelect}
      />
    ));
    const laterDocumentHandler = vi.fn();
    document.addEventListener('keydown', laterDocumentHandler);

    const enterEvent = new KeyboardEvent('keydown', {
      bubbles: true,
      cancelable: true,
      key: 'Enter',
    });
    document.dispatchEvent(enterEvent);

    document.removeEventListener('keydown', laterDocumentHandler);
    expect(enterEvent.defaultPrevented).toBe(true);
    expect(laterDocumentHandler).not.toHaveBeenCalled();
    expect(onSelect).not.toHaveBeenCalled();
    expect(screen.getByRole('status')).toHaveTextContent(
      'No Assistant commands match /not-a-command',
    );
  });

  it('does not render persistent keyboard shortcut hints', () => {
    render(() => (
      <SlashCommandAutocomplete
        query=""
        visible
        position={{ top: 58, left: 0 }}
        onClose={vi.fn()}
        onSelect={vi.fn()}
      />
    ));

    expect(screen.queryByText('up/down')).not.toBeInTheDocument();
    expect(screen.queryByText('navigate')).not.toBeInTheDocument();
    expect(screen.queryByText('enter')).not.toBeInTheDocument();
    expect(screen.queryByText('run')).not.toBeInTheDocument();
  });
});
