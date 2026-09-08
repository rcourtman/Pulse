import { createRoot } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';
import { useTypeToSearch } from '../useTypeToSearch';

describe('type-to-search keyboard ownership', () => {
  let dispose: (() => void) | undefined;
  afterEach(() => {
    dispose?.();
    document.body.replaceChildren();
  });

  const setup = () => {
    const input = document.createElement('input');
    document.body.append(input);
    createRoot((cleanup) => {
      dispose = cleanup;
      useTypeToSearch({ getInput: () => input });
    });
    return input;
  };

  it.each(['button', 'summary', 'role-button', 'button-child'])(
    'leaves Space to %s',
    async (kind) => {
      const input = setup();
      const control = document.createElement(
        kind === 'summary' ? 'summary' : kind === 'role-button' ? 'div' : 'button',
      );
      control.tabIndex = 0;
      if (kind === 'role-button') control.setAttribute('role', 'button');
      document.body.append(control);
      const target =
        kind === 'button-child' ? control.appendChild(document.createElement('span')) : control;
      control.focus();
      const event = new KeyboardEvent('keydown', { key: ' ', bubbles: true, cancelable: true });
      target.dispatchEvent(event);
      await Promise.resolve();
      expect(event.defaultPrevented).toBe(false);
      expect(document.activeElement).toBe(control);
      expect(input.value).toBe('');
    },
  );

  it('still captures ordinary typing and spaces away from controls', async () => {
    const input = setup();
    for (const key of ['a', ' ']) {
      input.blur();
      const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true });
      document.body.dispatchEvent(event);
      await Promise.resolve();
      expect(event.defaultPrevented).toBe(true);
      expect(document.activeElement).toBe(input);
    }
    expect(input.value).toBe('a ');
  });

  it.each([false, true])(
    'leaves modal keys alone when background is inert (prepared=%s)',
    async (prepared) => {
      const background = document.createElement('div');
      background.setAttribute('inert', '');
      const input = document.createElement('input');
      input.value = 'selected occurrence';
      background.append(input);
      const modalControl = document.createElement('button');
      document.body.append(background, modalControl);
      let preparations = 0;
      createRoot((cleanup) => {
        dispose = cleanup;
        useTypeToSearch({
          getInput: () => input,
          prepareInput: prepared
            ? () => {
                preparations++;
              }
            : undefined,
          clearOnEscape: true,
          focusOnShortcut: true,
          captureBackspace: true,
          getValue: () => input.value,
          onClear: () => {
            input.value = '';
          },
        });
      });
      modalControl.focus();
      for (const init of [
        { key: 'Escape' },
        { key: 'f', ctrlKey: true },
        { key: 'Backspace' },
        { key: 'x' },
      ]) {
        const event = new KeyboardEvent('keydown', { ...init, bubbles: true, cancelable: true });
        modalControl.dispatchEvent(event);
        await Promise.resolve();
        expect(event.defaultPrevented).toBe(false);
        expect(input.value).toBe('selected occurrence');
        expect(document.activeElement).toBe(modalControl);
      }
      expect(preparations).toBe(0);
      background.removeAttribute('inert');
      const escape = new KeyboardEvent('keydown', {
        key: 'Escape',
        bubbles: true,
        cancelable: true,
      });
      modalControl.dispatchEvent(escape);
      expect(escape.defaultPrevented).toBe(true);
      expect(input.value).toBe('');
    },
  );
});
