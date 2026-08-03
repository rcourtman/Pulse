import { Route, Router } from '@solidjs/router';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { SavedViewsMenu } from './SavedViewsMenu';

const STORAGE_PREFIX = 'pulse:filterbar:saved-views:';

const renderMenu = (storageKey: string) =>
  render(() => (
    <Router>
      <Route path="/" component={() => <SavedViewsMenu storageKey={storageKey} />} />
    </Router>
  ));

describe('SavedViewsMenu', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/');
    window.localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    window.localStorage.clear();
  });

  it('uses compound-popover dialog semantics and restores focus through Escape', async () => {
    renderMenu('focus-contract');

    const trigger = screen.getByRole('button', { name: 'Saved views' });
    expect(trigger).toHaveAttribute('aria-haspopup', 'dialog');

    fireEvent.click(trigger);
    expect(screen.getByRole('dialog', { name: 'Saved views' })).toBeInTheDocument();
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();

    const saveCurrent = screen.getByRole('button', { name: 'Save current view as...' });
    fireEvent.click(saveCurrent);

    const nameInput = screen.getByRole('textbox', { name: 'Name this view' });
    await waitFor(() => expect(nameInput).toHaveFocus());

    fireEvent.keyDown(document, { key: 'Escape' });
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Save current view as...' })).toHaveFocus(),
    );

    fireEvent.keyDown(document, { key: 'Escape' });
    await waitFor(() =>
      expect(screen.queryByRole('dialog', { name: 'Saved views' })).not.toBeInTheDocument(),
    );
    expect(trigger).toHaveFocus();
  });

  it('anchors from the leading edge on narrow rails and the trailing edge on desktop', () => {
    renderMenu('responsive-anchor-contract');

    fireEvent.click(screen.getByRole('button', { name: 'Saved views' }));

    expect(screen.getByRole('dialog', { name: 'Saved views' })).toHaveClass(
      'left-0',
      'right-auto',
      'md:left-auto',
      'md:right-0',
    );
  });

  it('resets draft state when the trigger dismisses the popover', async () => {
    renderMenu('dismiss-contract');

    const trigger = screen.getByRole('button', { name: 'Saved views' });
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole('button', { name: 'Save current view as...' }));
    fireEvent.input(screen.getByRole('textbox', { name: 'Name this view' }), {
      target: { value: 'Draft view' },
    });

    fireEvent.click(trigger);
    fireEvent.click(trigger);

    expect(screen.queryByRole('textbox', { name: 'Name this view' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save current view as...' })).toBeInTheDocument();
  });

  it('keeps saved-view management as visible buttons on narrow layouts', async () => {
    const storageKey = 'management-contract';
    window.localStorage.setItem(
      `${STORAGE_PREFIX}${storageKey}`,
      JSON.stringify([
        {
          id: 'review-temp',
          name: 'Review temp',
          query: 'status=stopped',
          savedAt: 1,
          version: 1,
        },
      ]),
    );

    renderMenu(storageKey);
    fireEvent.click(screen.getByRole('button', { name: 'Saved views' }));

    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Review temp' })).toBeInTheDocument(),
    );
    expect(
      screen.getByRole('button', { name: 'Set "Review temp" as default' }),
    ).toBeInTheDocument();

    const setDefault = screen.getByRole('button', { name: 'Set "Review temp" as default' });
    fireEvent.click(setDefault);
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Unset "Review temp" as default' })).toHaveFocus(),
    );

    const remove = screen.getByRole('button', { name: 'Remove view "Review temp"' });
    expect(remove).toHaveClass('opacity-100');
    fireEvent.click(remove);

    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Review temp' })).not.toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: 'Save current view as...' })).toHaveFocus();
  });
});
