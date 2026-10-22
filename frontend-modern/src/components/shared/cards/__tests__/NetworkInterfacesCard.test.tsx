import { cleanup, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';
import type { HostNetworkInterface } from '@/types/api';
import { NetworkInterfacesCard } from '../NetworkInterfacesCard';

afterEach(cleanup);
const interfaces = [{ name: 'eth0', addresses: ['192.0.2.11/24'] }];

describe('NetworkInterfacesCard observation scope', () => {
  it('keeps retained addresses visible but neutral with a touch-readable reason', () => {
    const [reason, setReason] = createSignal<string>();
    render(() => <NetworkInterfacesCard interfaces={interfaces} lastKnownReason={reason()} />);
    expect(screen.getByText('192.0.2.11/24')).toHaveClass('text-blue-700');
    setReason('Proxmox reports this VM paused');
    expect(screen.getByText('192.0.2.11/24')).toHaveClass('text-muted');
    expect(screen.getByRole('status')).toHaveTextContent(
      'Last known interface details, not a current connection. Proxmox reports this VM paused.',
    );
    setReason(undefined);
    expect(screen.queryByRole('status')).toBeNull();
    expect(screen.getByText('192.0.2.11/24')).toHaveClass('text-blue-700');
  });
  it('reacts to the first report and its removal in an already-open drawer', () => {
    const [values, setValues] = createSignal<HostNetworkInterface[]>();
    render(() => <NetworkInterfacesCard interfaces={values()} />);
    expect(screen.queryByText('Network')).toBeNull();
    setValues(interfaces);
    expect(screen.getByText('eth0')).toBeInTheDocument();
    setValues([]);
    expect(screen.queryByText('Network')).toBeNull();
  });
});
