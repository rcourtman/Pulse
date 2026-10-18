import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { PlatformTableToolbar } from '../sharedPlatformPage';
import { setKioskMode } from '@/utils/url';

const renderToolbar = () =>
  render(() => (
    <PlatformTableToolbar
      search={() => ''}
      onSearchChange={vi.fn()}
      searchPlaceholder="Search containers"
      status="all"
      onStatusChange={vi.fn()}
      statusOptions={[
        { value: 'all', label: 'All' },
        { value: 'healthy', label: 'Healthy' },
      ]}
      visible={3}
      total={3}
      rowNoun="container"
    />
  ));

describe('PlatformTableToolbar in kiosk mode', () => {
  afterEach(() => {
    setKioskMode(false);
    window.sessionStorage.removeItem('pulse_kiosk_mode');
    cleanup();
  });

  it('stays off a kiosk display so no platform table carries search or filters, and returns after kiosk', () => {
    setKioskMode(true);
    renderToolbar();

    expect(screen.queryByPlaceholderText('Search containers')).not.toBeInTheDocument();
    expect(screen.queryByRole('group', { name: 'Status' })).not.toBeInTheDocument();

    setKioskMode(false);

    expect(screen.getByPlaceholderText('Search containers')).toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'Status' })).toBeInTheDocument();
  });
});
