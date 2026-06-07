import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { ASSISTANT_SLASH_COMMANDS } from '../assistantSlashCommands';
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
        action: 'redo',
        name: 'redo',
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

    const options = screen.getAllByRole('option');
    expect(options).toHaveLength(ASSISTANT_SLASH_COMMANDS.length);
    expect(options.map((option) => option.getAttribute('aria-label'))).toContain(
      'Run /compact: Summarize older turns and keep this session moving',
    );
    expect(container.querySelectorAll('[role="option"] svg')).toHaveLength(
      ASSISTANT_SLASH_COMMANDS.length,
    );
  });

  it('omits disabled commands from prompt autocomplete', () => {
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

    expect(screen.queryByRole('listbox', { name: 'Assistant commands' })).not.toBeInTheDocument();
    expect(screen.queryByRole('option', { name: /\/compact/ })).not.toBeInTheDocument();
  });
});
