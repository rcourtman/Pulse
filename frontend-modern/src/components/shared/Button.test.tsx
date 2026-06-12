import { Route, Router } from '@solidjs/router';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Button, ButtonLink, CommandCopyButton, CopyValueButton } from './Button';
import buttonSource from './Button.tsx?raw';
import buttonModelSource from './buttonModel.ts?raw';
import { CopyableCodeRow } from './CopyableCodeRow';
import copyableCodeRowSource from './CopyableCodeRow.tsx?raw';

describe('Button', () => {
  afterEach(() => {
    cleanup();
    window.history.replaceState({}, '', '/');
  });

  it('keeps shell styling in the shared model', () => {
    expect(buttonSource).toContain('getButtonClass');
    expect(buttonModelSource).toContain('export const BUTTON_VARIANT_CLASSES');
    expect(buttonModelSource).toContain('export const COPY_VALUE_BUTTON_VARIANT_CLASSES');
    expect(buttonModelSource).toContain('export const COPY_VALUE_BUTTON_SIZE_CLASSES');
    expect(buttonModelSource).toContain('getCopyValueButtonClass');
    expect(buttonModelSource).toContain(
      "secondary: 'border border-border bg-surface text-base-content shadow-sm hover:bg-surface-hover'",
    );
    expect(buttonModelSource).toContain('primaryFlat:');
    expect(buttonModelSource).toContain(
      "'border border-border bg-surface text-muted hover:bg-surface-hover hover:text-base-content'",
    );
    expect(buttonModelSource).toContain(
      "accent: 'text-blue-700 hover:bg-blue-100 dark:text-blue-200 dark:hover:bg-blue-950'",
    );
    expect(buttonModelSource).toContain('dangerOutline:');
    expect(buttonModelSource).toContain('export const BUTTON_SIZE_CLASSES');
    expect(buttonModelSource).toContain("xs: 'px-2.5 py-1 text-xs'");
    expect(buttonModelSource).toContain("mdCompact: 'px-3 py-2 text-sm'");
    expect(buttonModelSource).toContain("settingsAction: 'min-h-10 px-3 py-2 text-sm sm:min-h-9'");
    expect(buttonModelSource).toContain("chip: 'gap-1 px-1.5 py-0.5 text-[10px]'");
    expect(buttonModelSource).toContain("iconMd: 'h-9 w-9 p-0'");
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

  it('renders settings action buttons through named size and variants', () => {
    render(() => (
      <>
        <Button variant="primary" size="settingsAction">
          Save source
        </Button>
        <Button variant="dangerOutline" size="settingsAction">
          Remove source
        </Button>
      </>
    ));

    const saveButton = screen.getByRole('button', { name: 'Save source' });
    expect(saveButton).toHaveClass('min-h-10');
    expect(saveButton).toHaveClass('sm:min-h-9');
    expect(saveButton).toHaveClass('bg-blue-600');

    const removeButton = screen.getByRole('button', { name: 'Remove source' });
    expect(removeButton).toHaveClass('min-h-10');
    expect(removeButton).toHaveClass('border-rose-300');
    expect(removeButton).toHaveClass('text-rose-700');
  });

  it('renders command-copy icon buttons through the shared primitive', () => {
    const onClick = vi.fn();

    render(() => <CommandCopyButton onClick={onClick} label="Copy install command" />);

    const button = screen.getByRole('button', { name: 'Copy install command' });
    expect(button).toHaveAttribute('type', 'button');
    expect(button).toHaveAttribute('title', 'Copy install command');
    expect(button).toHaveClass('absolute');
    expect(button).toHaveClass('right-2');
    expect(button).toHaveClass('top-2');
    expect(button).toHaveClass('min-h-10');
    expect(button).toHaveClass('min-w-10');
    expect(button).toHaveClass('bg-surface-hover');
    expect(button).toHaveClass('p-2');

    button.click();
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('renders copy-value icon and chip buttons through the shared primitive', () => {
    const onCopy = vi.fn();

    render(() => (
      <>
        <CopyValueButton
          value="  https://example.test  "
          copied={false}
          onCopyValue={onCopy}
          label="Copy URL"
        />
        <CopyValueButton
          value="8443/tcp"
          copied
          onCopyValue={onCopy}
          label="Copy 8443/tcp"
          variant="chip"
          size="chip"
        >
          <span>8443/tcp</span>
        </CopyValueButton>
        <CopyValueButton value=" " onCopyValue={onCopy} label="Copy blank" />
      </>
    ));

    const copyUrlButton = screen.getByRole('button', { name: 'Copy URL' });
    expect(copyUrlButton).toHaveClass('border-border');
    expect(copyUrlButton).toHaveClass('min-h-7');
    copyUrlButton.click();
    expect(onCopy).toHaveBeenCalledWith('https://example.test');

    const chipButton = screen.getByRole('button', { name: 'Copy 8443/tcp' });
    expect(chipButton).toHaveClass('bg-surface-alt');
    expect(chipButton).toHaveClass('text-[10px]');

    expect(screen.getByRole('button', { name: 'Copy blank' })).toBeDisabled();
  });

  it('renders copyable code rows through the shared copy primitive', () => {
    const onCopy = vi.fn();

    render(() => (
      <CopyableCodeRow
        value="/etc/pulse/config.yml"
        copied={false}
        onCopyValue={onCopy}
        label="Copy config path"
      />
    ));

    expect(copyableCodeRowSource).toContain('CopyValueButton');
    expect(screen.getByText('/etc/pulse/config.yml')).toHaveClass('font-mono');

    const copyButton = screen.getByRole('button', { name: 'Copy config path' });
    expect(copyButton).toHaveClass('min-h-6');
    copyButton.click();
    expect(onCopy).toHaveBeenCalledWith('/etc/pulse/config.yml');
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
