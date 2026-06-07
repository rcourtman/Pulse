import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@solidjs/testing-library';
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

  it('wraps keyboard selection from the first command to the last visible command', () => {
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
        action: 'export',
        name: 'export',
      }),
    );
  });
});
