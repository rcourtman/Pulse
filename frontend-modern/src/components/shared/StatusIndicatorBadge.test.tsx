import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { StatusIndicatorBadge } from '@/components/shared/StatusIndicatorBadge';

afterEach(cleanup);

describe('StatusIndicatorBadge', () => {
  it('renders canonical read-only health badge presentation from status text', () => {
    render(() => <StatusIndicatorBadge status="online" dot uppercase size="xs" />);

    const badge = screen.getByText('Online');
    expect(badge).toHaveClass('inline-flex');
    expect(badge).toHaveClass('rounded-full');
    expect(badge).toHaveClass('text-[10px]');
    expect(badge).toHaveClass('uppercase');
    expect(badge).toHaveClass('bg-green-100');
    expect(badge.querySelector('[aria-hidden="true"]')).toHaveClass('bg-emerald-500');
  });

  it('renders custom state labels without forcing a dot or uppercase text', () => {
    render(() => (
      <StatusIndicatorBadge variant="warning" label="Cooldown: Missing" size="md" shape="rounded" />
    ));

    const badge = screen.getByText('Cooldown: Missing');
    expect(badge).toHaveClass('rounded');
    expect(badge).toHaveClass('py-1');
    expect(badge).toHaveClass('bg-amber-100');
    expect(badge.querySelector('[aria-hidden="true"]')).toBeNull();
    expect(badge).not.toHaveClass('uppercase');
  });

  it('renders informational state badges through the shared variant catalog', () => {
    render(() => (
      <StatusIndicatorBadge label="Run in progress" variant="info" size="xs" shape="rounded" />
    ));

    const badge = screen.getByText('Run in progress');
    expect(badge).toHaveClass('rounded');
    expect(badge).toHaveClass('text-[10px]');
    expect(badge).toHaveClass('bg-blue-100');
    expect(badge).toHaveClass('text-blue-700');
  });
});
