import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { InlineNotice } from './InlineNotice';

describe('InlineNotice', () => {
  afterEach(() => {
    cleanup();
  });

  it('renders dense status copy, icon, and action link through the shared primitive', () => {
    render(() => (
      <InlineNotice
        role="status"
        icon={<span data-testid="notice-icon">!</span>}
        actionHref="/settings/infrastructure"
        actionLabel="Open Infrastructure settings"
        actionIcon={<span data-testid="action-icon">go</span>}
      >
        Update the affected agent to see inventory.
      </InlineNotice>
    ));

    expect(screen.getByRole('status')).toHaveTextContent(
      'Update the affected agent to see inventory.',
    );
    expect(screen.getByTestId('notice-icon')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open Infrastructure settings' })).toHaveAttribute(
      'href',
      '/settings/infrastructure',
    );
    expect(screen.getByTestId('action-icon')).toBeInTheDocument();
  });

  it('owns warning notice and action-link tone classes', () => {
    render(() => (
      <InlineNotice actionHref="/settings/infrastructure" actionLabel="Open settings">
        Shared warning notice.
      </InlineNotice>
    ));

    const notice = screen.getByText('Shared warning notice.').closest('.rounded-lg');
    expect(notice?.className).toContain('border-amber-300');
    expect(notice?.className).toContain('bg-amber-50');
    expect(screen.getByRole('link', { name: 'Open settings' }).className).toContain(
      'text-amber-900',
    );
  });

  it('supports non-warning platform notice tones without changing consumers', () => {
    render(() => <InlineNotice tone="info">Shared informational notice.</InlineNotice>);

    const notice = screen.getByText('Shared informational notice.').closest('.rounded-lg');
    expect(notice?.className).toContain('border-blue-300');
  });
});
