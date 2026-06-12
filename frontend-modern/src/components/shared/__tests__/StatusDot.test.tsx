import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { StatusDot } from '@/components/shared/StatusDot';

afterEach(cleanup);

describe('StatusDot', () => {
  it('renders shared variant, size, pulse, and accessible status semantics', () => {
    render(() => (
      <StatusDot variant="warning" size="md" pulse title="Attention" ariaLabel="Attention" />
    ));

    const dot = screen.getByRole('img', { name: 'Attention' });

    expect(dot).toHaveClass('inline-block');
    expect(dot).toHaveClass('rounded-full');
    expect(dot).toHaveClass('flex-shrink-0');
    expect(dot).toHaveClass('h-2.5');
    expect(dot).toHaveClass('w-2.5');
    expect(dot).toHaveClass('bg-amber-500');
    expect(dot).toHaveClass('animate-pulse');
    expect(dot).toHaveAttribute('title', 'Attention');
  });

  it('hides decorative indicators from assistive tech by default', () => {
    render(() => <StatusDot variant="success" size="xs" />);

    const dot = document.querySelector('span');

    expect(dot).toHaveClass('h-1.5');
    expect(dot).toHaveClass('w-1.5');
    expect(dot).toHaveClass('bg-emerald-500');
    expect(dot).toHaveAttribute('aria-hidden', 'true');
    expect(dot).not.toHaveAttribute('role');
  });

  it('renders informational indicators from the shared variant catalog', () => {
    render(() => <StatusDot variant="info" ariaLabel="Informational" />);

    const dot = screen.getByRole('img', { name: 'Informational' });

    expect(dot).toHaveClass('bg-blue-500');
  });
});
