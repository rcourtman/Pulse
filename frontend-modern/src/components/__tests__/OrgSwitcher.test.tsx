import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import type { Organization } from '@/api/orgs';
import { OrgSwitcher } from '@/components/OrgSwitcher';

describe('OrgSwitcher', () => {
  it('renders current org and calls onChange when switched', async () => {
    const onChange = vi.fn();

    render(() => (
      <OrgSwitcher
        orgs={[
          { id: 'default', displayName: 'Default Organization' },
          { id: 'acme', displayName: 'Acme Corp' },
        ]}
        selectedOrgId="default"
        onChange={onChange}
      />
    ));

    const select = screen.getByLabelText('Organization');
    expect(select).toBeInTheDocument();

    fireEvent.change(select, { target: { value: 'acme' } });
    expect(onChange).toHaveBeenCalledWith('acme');
  });

  it('shows static org label when only one org exists', () => {
    render(() => (
      <OrgSwitcher
        orgs={[{ id: 'default', displayName: 'Default Organization' }]}
        selectedOrgId="default"
        onChange={() => {}}
      />
    ));

    expect(screen.getByText('Default Organization')).toBeInTheDocument();
    expect(screen.queryByLabelText('Organization')).toBeNull();
  });

  it('disambiguates same-named choices and describes the exact selected organization', () => {
    const [selected, setSelected] = createSignal('west');
    const onChange = vi.fn((id: string) => setSelected(id));
    render(() => (
      <OrgSwitcher
        orgs={[
          { id: 'west', displayName: 'Customer operations' },
          { id: 'east', displayName: 'Customer operations' },
          { id: 'internal', displayName: 'Internal' },
        ]}
        selectedOrgId={selected()}
        onChange={onChange}
      />
    ));

    const select = screen.getByRole('combobox', { name: 'Organization' });
    expect(screen.getByRole('option', { name: 'Customer operations (west)' })).toHaveValue('west');
    expect(screen.getByRole('option', { name: 'Customer operations (east)' })).toHaveValue('east');
    expect(screen.getByRole('option', { name: 'Internal' })).toHaveValue('internal');
    expect(select).toHaveAccessibleDescription('Organization ID: west');
    expect(screen.getByText('west', { selector: 'bdi' })).toBeVisible();

    fireEvent.change(select, { target: { value: 'east' } });
    expect(onChange).toHaveBeenCalledExactlyOnceWith('east');
    expect(select).toHaveValue('east');
    expect(select).toHaveAccessibleDescription('Organization ID: east');
    expect(screen.queryByText('west', { selector: 'bdi' })).not.toBeInTheDocument();

    fireEvent.change(select, { target: { value: 'internal' } });
    expect(select).not.toHaveAttribute('aria-describedby');
    expect(select).not.toHaveAccessibleDescription();
  });

  it('recognizes colliding names after native option whitespace normalization and ID fallback', () => {
    render(() => (
      <OrgSwitcher
        orgs={[
          { id: 'west', displayName: ' Customer   operations ' },
          { id: 'fallback', displayName: '' },
          { id: 'named-fallback', displayName: 'fallback' },
          { id: 'east', displayName: 'Customer operations' },
        ]}
        selectedOrgId="west"
        onChange={() => {}}
      />
    ));

    expect(screen.getByRole('option', { name: 'Customer operations (west)' })).toHaveValue('west');
    expect(screen.getByRole('option', { name: 'fallback (fallback)' })).toHaveValue('fallback');
    expect(screen.getByRole('option', { name: 'fallback (named-fallback)' })).toHaveValue(
      'named-fallback',
    );
    expect(screen.getByRole('option', { name: 'Customer operations (east)' })).toHaveValue('east');
    expect(screen.getByRole('combobox')).toHaveAccessibleDescription('Organization ID: west');
  });

  it('withdraws stale disambiguation when the supplied organization list changes', () => {
    const [orgs, setOrgs] = createSignal<Organization[]>([
      { id: 'west', displayName: 'Customer operations' },
      { id: 'east', displayName: 'Customer operations' },
    ]);
    const onChange = vi.fn();
    render(() => <OrgSwitcher orgs={orgs()} selectedOrgId="west" onChange={onChange} />);

    const select = screen.getByRole('combobox', { name: 'Organization' });
    expect(select).toHaveAccessibleDescription('Organization ID: west');
    setOrgs([
      { id: 'west', displayName: 'Customer operations' },
      { id: 'east', displayName: 'Other customer' },
    ]);
    expect(select).not.toHaveAttribute('aria-describedby');
    expect(screen.getByRole('option', { name: 'Customer operations' })).toHaveValue('west');
    expect(screen.queryByText('west', { selector: 'bdi' })).not.toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();

    setOrgs([{ id: 'west', displayName: 'Customer operations' }]);
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    expect(screen.getByText('Customer operations')).toBeVisible();
    expect(onChange).not.toHaveBeenCalled();
  });

  it('keeps the current identity visible while switching is disabled', () => {
    const onChange = vi.fn();
    render(() => (
      <OrgSwitcher
        orgs={[
          { id: 'west', displayName: 'Customer operations' },
          { id: 'east', displayName: 'Customer operations' },
        ]}
        selectedOrgId="east"
        loading
        onChange={onChange}
      />
    ));
    const select = screen.getByRole('combobox', { name: 'Organization' });
    expect(select).toBeDisabled();
    expect(select).toHaveValue('east');
    expect(select).toHaveAccessibleDescription('Organization ID: east');
    expect(screen.getByText('east', { selector: 'bdi' })).toBeVisible();
    expect(onChange).not.toHaveBeenCalled();
  });

  it('keeps a focus anchor across a read-only retry of unavailable choices', () => {
    const [issue, setIssue] = createSignal<'unavailable' | null>('unavailable');
    const [loading, setLoading] = createSignal(false);
    const onChange = vi.fn();
    const onRetry = vi.fn(() => setLoading(true));
    render(() => (
      <OrgSwitcher
        orgs={[
          { id: 'west', displayName: 'Operations' },
          { id: 'east', displayName: 'Operations' },
        ]}
        selectedOrgId="west"
        loading={loading()}
        listIssue={issue()}
        onChange={onChange}
        onRetry={onRetry}
      />
    ));

    const status = screen.getByRole('status');
    expect(status).toHaveTextContent('Organization list unavailable');
    expect(status).toHaveTextContent('Selected ID: west');
    expect(screen.queryByRole('combobox')).toBeNull();
    const retry = screen.getByRole('button', { name: 'Retry organization list' });
    retry.focus();
    fireEvent.click(retry);
    const group = screen.getByRole('group', { name: 'Organization selection' });
    expect(group).toHaveFocus();
    expect(onRetry).toHaveBeenCalledOnce();
    expect(retry).toBeDisabled();
    expect(status).toHaveTextContent('Checking organizations…');
    setIssue(null);
    setLoading(false);
    expect(group).toHaveFocus();
    expect(screen.getByRole('combobox', { name: 'Organization' })).toHaveValue('west');
    expect(onChange).not.toHaveBeenCalled();
  });

  it('reports a verified empty list without claiming an unbound session selected Default', () => {
    render(() => (
      <OrgSwitcher
        orgs={[]}
        selectedOrgId=""
        listIssue="empty"
        onChange={() => {}}
        onRetry={() => {}}
      />
    ));
    expect(screen.getByRole('status')).toHaveTextContent('No organizations listed');
    expect(screen.getByRole('status')).not.toHaveTextContent('Selected ID');
    expect(screen.queryByText('Default')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry organization list' })).toBeEnabled();
  });
});
