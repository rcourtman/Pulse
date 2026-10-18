import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { NetworkInfoCell } from '../GuestRowCells';
import { getWorkloadsGuestNetworkEmptyState } from '@/utils/workloadGuestPresentation';

vi.mock('@/hooks/useTooltip', () => ({
  useTooltip: () => ({
    show: () => true,
    pos: () => ({ x: 100, y: 100 }),
    onMouseEnter: () => {},
    onMouseLeave: () => {},
  }),
}));
vi.mock('@/components/shared/hoverCapability', () => ({ supportsHoverTooltips: () => true }));
afterEach(cleanup);

describe('guest network tooltip palette', () => {
  it('keeps names, addresses and supporting readings on the semantic palette', () => {
    render(() => (
      <NetworkInfoCell
        ipAddresses={['192.0.2.80']}
        networkInterfaces={[
          {
            name: 'eth0',
            addresses: ['192.0.2.80'],
            mac: '02:00:00:00:00:80',
            rxBytes: 1024,
            txBytes: 2048,
          },
          { name: 'eth1', addresses: [] },
        ]}
      />
    ));
    const header = screen.getByText('Network Interfaces');
    const tip = within(header.closest('[data-tooltip-portal]') as HTMLElement);
    expect(header).toHaveClass('text-base-content');
    expect(tip.getByText('eth0').parentElement).toHaveClass('text-base-content');
    expect(tip.getByText('192.0.2.80')).toHaveClass('text-base-content');
    expect(tip.getByText('02:00:00:00:00:80')).toHaveClass('text-muted');
    expect(tip.getByText(getWorkloadsGuestNetworkEmptyState())).toHaveClass('text-muted');
    expect(tip.getByText(/RX: 1.00 KB/)).toHaveClass('text-muted');
  });

  it('keeps the address-only fallback readable without changing its values', () => {
    render(() => (
      <NetworkInfoCell ipAddresses={['192.0.2.80', '10.88.0.1']} networkInterfaces={[]} />
    ));
    expect(screen.getByText('IP Addresses').parentElement).toHaveClass('text-base-content');
    expect(screen.getByText('No agent data')).toHaveClass('text-muted');
    for (const ip of ['192.0.2.80', '10.88.0.1'])
      expect(screen.getByText(ip)).toHaveClass('text-base-content');
  });
});
