import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { MetadataBadge, getMetadataBadgeClass } from '@/components/shared/MetadataBadge';

afterEach(cleanup);

describe('MetadataBadge', () => {
  it('renders the canonical read-only badge shell with typed tone, size, and shape', () => {
    render(() => (
      <MetadataBadge tone="warning" size="xs" shape="rounded" uppercase title="Pending review">
        pending
      </MetadataBadge>
    ));

    const badge = screen.getByText('pending');
    expect(badge).toHaveClass('inline-flex');
    expect(badge).toHaveClass('rounded');
    expect(badge).toHaveClass('text-[10px]');
    expect(badge).toHaveClass('uppercase');
    expect(badge).toHaveClass('bg-amber-100');
    expect(badge).toHaveAttribute('title', 'Pending review');
  });

  it('keeps class composition in the shared primitive model', () => {
    expect(getMetadataBadgeClass({ tone: 'success', fit: true, class: 'shrink-0' })).toContain(
      'w-fit',
    );
    expect(getMetadataBadgeClass({ tone: 'success', fit: true, class: 'shrink-0' })).toContain(
      'bg-emerald-100',
    );
    expect(getMetadataBadgeClass({ tone: 'success', fit: true, class: 'shrink-0' })).toContain(
      'shrink-0',
    );
  });

  it('owns outlined badge appearance for compact metadata rows', () => {
    const badgeClass = getMetadataBadgeClass({
      tone: 'orange',
      appearance: 'outline',
      size: 'xs',
      shape: 'rounded',
    });

    expect(badgeClass).toContain('border');
    expect(badgeClass).toContain('border-orange-200');
    expect(badgeClass).toContain('bg-orange-50');
    expect(badgeClass).toContain('text-orange-700');
    expect(badgeClass).toContain('text-[10px]');
  });
});
