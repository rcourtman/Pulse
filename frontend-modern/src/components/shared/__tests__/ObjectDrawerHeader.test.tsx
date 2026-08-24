import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ObjectDrawerHeader } from '../ObjectDrawerHeader';

describe('ObjectDrawerHeader', () => {
  afterEach(() => cleanup());

  it('makes the full header surface a keyboard-accessible collapse control', async () => {
    const onCollapse = vi.fn();

    render(() => (
      <ObjectDrawerHeader collapseLabel="Collapse alpha details" onCollapse={onCollapse}>
        <h2>alpha</h2>
      </ObjectDrawerHeader>
    ));

    const collapse = screen.getByRole('button', { name: 'Collapse alpha details' });
    expect(collapse).toHaveClass('absolute', 'inset-0');

    await fireEvent.click(collapse);
    expect(onCollapse).toHaveBeenCalledTimes(1);

    collapse.focus();
    expect(collapse).toHaveFocus();
    expect(collapse.tagName).toBe('BUTTON');
  });

  it('keeps object-specific header actions independent from collapse', async () => {
    const onCollapse = vi.fn();
    const onAction = vi.fn();

    render(() => (
      <ObjectDrawerHeader
        collapseLabel="Collapse container details"
        onCollapse={onCollapse}
        actions={<button onClick={onAction}>Restart</button>}
      >
        <h2>container</h2>
      </ObjectDrawerHeader>
    ));

    await fireEvent.click(screen.getByRole('button', { name: 'Restart' }));
    expect(onAction).toHaveBeenCalledTimes(1);
    expect(onCollapse).not.toHaveBeenCalled();
  });
});
