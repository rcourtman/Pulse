import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@solidjs/testing-library';
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
});
