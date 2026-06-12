import { Route, Router } from '@solidjs/router';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Button, ButtonLink } from './Button';
import buttonSource from './Button.tsx?raw';
import buttonModelSource from './buttonModel.ts?raw';

describe('Button', () => {
  afterEach(() => {
    cleanup();
    window.history.replaceState({}, '', '/');
  });

  it('keeps shell styling in the shared model', () => {
    expect(buttonSource).toContain('getButtonClass');
    expect(buttonModelSource).toContain('export const BUTTON_VARIANT_CLASSES');
    expect(buttonModelSource).toContain(
      "secondary: 'border border-border bg-surface text-base-content shadow-sm hover:bg-surface-hover'",
    );
    expect(buttonModelSource).toContain('export const BUTTON_SIZE_CLASSES');
    expect(buttonModelSource).toContain("xs: 'px-2.5 py-1 text-xs'");
    expect(buttonModelSource).toContain("mdCompact: 'px-3 py-2 text-sm'");
  });

  it('renders command buttons with the shared secondary shell', () => {
    const onClick = vi.fn();

    render(() => (
      <Button variant="secondary" size="sm" class="gap-2 px-3" onClick={onClick}>
        Add agent
      </Button>
    ));

    const button = screen.getByRole('button', { name: 'Add agent' });
    expect(button).toHaveAttribute('type', 'button');
    expect(button).toHaveClass('bg-surface');
    expect(button).toHaveClass('border-border');
    expect(button).toHaveClass('px-3');

    button.click();
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('disables loading command buttons through the shared primitive', () => {
    render(() => (
      <Button variant="secondary" size="sm" isLoading>
        Refresh
      </Button>
    ));

    expect(screen.getByRole('button', { name: 'Refresh' })).toBeDisabled();
  });

  it('renders in-app button links through the router', () => {
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <ButtonLink href="/standalone/availability" size="sm">
              View checks
            </ButtonLink>
          )}
        />
      </Router>
    ));

    const link = screen.getByRole('link', { name: 'View checks' });
    expect(link).toHaveAttribute('href', '/standalone/availability');
    expect(link).not.toHaveAttribute('target');
    expect(link).toHaveClass('bg-surface');
  });

  it('renders external or new-tab button links as safe native anchors', () => {
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <ButtonLink href="https://example.com/docs" target="_blank" size="sm">
              Docs
            </ButtonLink>
          )}
        />
      </Router>
    ));

    const link = screen.getByRole('link', { name: 'Docs' });
    expect(link).toHaveAttribute('href', 'https://example.com/docs');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });
});
