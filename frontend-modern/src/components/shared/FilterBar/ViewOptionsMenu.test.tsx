import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import { ViewOptionsMenu } from './ViewOptionsMenu';

afterEach(cleanup);

describe('ViewOptionsMenu', () => {
  it('keeps persistent presentation controls behind one View trigger', () => {
    render(() => (
      <ViewOptionsMenu>
        <button type="button">Grouped</button>
      </ViewOptionsMenu>
    ));

    expect(screen.queryByRole('dialog', { name: 'View preferences' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'View' }));

    expect(screen.getByRole('dialog', { name: 'View preferences' })).toBeInTheDocument();
    expect(screen.getByRole('dialog', { name: 'View preferences' })).toHaveClass(
      'filter-bottom-nav-aware-panel',
    );
    expect(screen.getByRole('button', { name: 'View' }).parentElement).toHaveClass(
      'static',
      'ml-auto',
      'sm:relative',
      'sm:ml-0',
    );
    expect(screen.getByRole('button', { name: 'Grouped' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'View' })).toHaveAttribute('aria-expanded', 'true');
  });

  it('closes on Escape and restores focus to the trigger', async () => {
    render(() => <ViewOptionsMenu>Preferences</ViewOptionsMenu>);
    const trigger = screen.getByRole('button', { name: 'View' });

    fireEvent.click(trigger);
    fireEvent.keyDown(document, { key: 'Escape' });
    await Promise.resolve();

    expect(screen.queryByRole('dialog', { name: 'View preferences' })).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it('closes when focus moves to an outside pointer target', () => {
    render(() => (
      <>
        <ViewOptionsMenu>Preferences</ViewOptionsMenu>
        <button type="button">Outside</button>
      </>
    ));

    fireEvent.click(screen.getByRole('button', { name: 'View' }));
    fireEvent.mouseDown(screen.getByRole('button', { name: 'Outside' }));

    expect(screen.queryByRole('dialog', { name: 'View preferences' })).not.toBeInTheDocument();
  });
});
