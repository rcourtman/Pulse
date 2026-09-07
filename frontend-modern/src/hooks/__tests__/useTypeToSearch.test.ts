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
});
