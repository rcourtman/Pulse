import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { Button } from './Button';
import { EmptyState } from './EmptyState';

describe('EmptyState', () => {
  afterEach(() => {
    cleanup();
  });

  it('renders the default framed empty-state shell', () => {
    render(() => (
      <EmptyState
        icon={<span data-testid="empty-icon">i</span>}
        title="No rows"
        description="There is nothing to show yet."
      />
    ));

    const shell = screen.getByText('No rows').closest('div');
    expect(screen.getByTestId('empty-icon')).toBeInTheDocument();
    expect(shell?.className).toContain('border-dashed');
    expect(shell?.className).toContain('sm:py-16');
  });

  it('renders compact panel empty states without the framed table shell', () => {
    render(() => (
      <EmptyState
        variant="panel"
        icon={<span data-testid="panel-icon">p</span>}
        title="No providers"
        description="Add a provider to continue."
        actions={<Button size="mdCompact">Add provider</Button>}
      />
    ));

    const shell = screen.getByText('No providers').closest('div');
    expect(screen.getByTestId('panel-icon')).toBeInTheDocument();
    expect(screen.getByText('No providers').className).toContain('text-sm');
    expect(screen.getByText('Add a provider to continue.').className).toContain('max-w-xl');
    expect(shell?.className).toContain('py-8');
    expect(shell?.className).not.toContain('border-dashed');
    expect(screen.getByRole('button', { name: 'Add provider' })).toBeInTheDocument();
  });

  it('supports left-aligned panel empty states', () => {
    render(() => <EmptyState variant="panel" align="left" title="No matches" />);

    const shell = screen.getByText('No matches').closest('div');
    expect(shell?.className).toContain('items-start');
    expect(shell?.className).toContain('text-left');
  });
});
