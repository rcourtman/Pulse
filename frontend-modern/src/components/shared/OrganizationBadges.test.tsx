import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import {
  getOrganizationRoleBadgeTone,
  getOrganizationShareStatusBadgeTone,
  OrganizationRoleBadge,
  OrganizationShareStatusBadge,
} from '@/components/shared/OrganizationBadges';

afterEach(cleanup);

describe('OrganizationBadges', () => {
  it('maps organization roles onto shared metadata badge tones', () => {
    expect(getOrganizationRoleBadgeTone('owner')).toBe('warning');
    expect(getOrganizationRoleBadgeTone('admin')).toBe('info');
    expect(getOrganizationRoleBadgeTone('editor')).toBe('success');
    expect(getOrganizationRoleBadgeTone('viewer')).toBe('muted');
    expect(getOrganizationRoleBadgeTone('unexpected')).toBe('muted');

    render(() => <OrganizationRoleBadge role="admin" />);

    const badge = screen.getByText('admin');
    expect(badge).toHaveClass('rounded-full');
    expect(badge).toHaveClass('bg-blue-100');
  });

  it('maps organization share status labels onto shared metadata badge tones', () => {
    expect(getOrganizationShareStatusBadgeTone('pending')).toBe('warning');
    expect(getOrganizationShareStatusBadgeTone('accepted')).toBe('success');

    render(() => <OrganizationShareStatusBadge status="accepted" />);

    const badge = screen.getByText('Active');
    expect(badge).toHaveClass('rounded-full');
    expect(badge).toHaveClass('w-fit');
    expect(badge).toHaveClass('bg-emerald-100');
  });
});
