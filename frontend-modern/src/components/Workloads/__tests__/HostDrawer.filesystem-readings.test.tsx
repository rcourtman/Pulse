import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { WebSocketContext, type WebSocketStore } from '@/contexts/appRuntime';
import { afterEach, describe, expect, it } from 'vitest';
import type { Disk, Node } from '@/types/api';
import type { Resource } from '@/types/resource';
import { NodeDrawerOverview } from '../NodeDrawerOverview';
import { DockerHostDrawerOverview } from '@/features/docker/DockerHostDrawerOverview';

const GB = 1024 ** 3;
const disks: Disk[] = [
  { mountpoint: '/unknown', total: 100 * GB, usage: -1 },
  { mountpoint: '/no-capacity', used: 30 * GB },
  { mountpoint: '/empty', total: 100 * GB, free: 100 * GB },
  { mountpoint: '/critical', total: 100 * GB, used: 95 * GB },
  { mountpoint: '/boot/efi', total: 100 * GB, used: 20 * GB },
];
const node = {
  id: 'fixture-node',
  name: 'fixture-node',
  status: 'online',
  instance: 'fixture',
  type: 'node',
} as Node;
afterEach(cleanup);

for (const kind of ['Proxmox node', 'Docker host'] as const) {
  describe(`${kind} per-filesystem presentation`, () => {
    const mount = () => {
      const [value, setValue] = createSignal(disks);
      render(() => (
        <WebSocketContext.Provider value={{ activeAlerts: {} } as unknown as WebSocketStore}>
          {kind === 'Proxmox node' ? (
            <NodeDrawerOverview node={node} disks={value()} />
          ) : (
            <DockerHostDrawerOverview
              host={
                {
                  id: 'fixture-docker',
                  type: 'docker-host',
                  name: 'fixture-docker',
                  agent: { disks: value() },
                } as Resource
              }
            />
          )}
        </WebSocketContext.Provider>
      ));
      return setValue;
    };

    it('shows missing usage neutrally, preserves capacity, and keeps measured zero/critical readings', () => {
      mount();
      const unknown = screen.getByText('/unknown').closest('tr')!;
      expect(unknown).toHaveTextContent('Usage unavailable · 100 GB total');
      expect(
        within(unknown).getByText('Usage unavailable · 100 GB total').closest('td'),
      ).toHaveClass('text-muted');
      expect(screen.getByText('/no-capacity').closest('tr')).toHaveTextContent('Usage unavailable');
      expect(screen.getByText('/empty').closest('tr')).toHaveTextContent('0% · 0 B / 100 GB');
      expect(screen.getByText('/critical').closest('tr')).toHaveTextContent(
        '95% · 95.0 GB / 100 GB',
      );
      expect(screen.queryByText('/boot/efi')).not.toBeInTheDocument();
    });

    it('replaces unavailable/current evidence on the mounted drawer without affecting peers', () => {
      const setValue = mount();
      setValue(
        disks.map((disk) =>
          disk.mountpoint === '/unknown' ? { ...disk, usage: 70, used: 70 * GB } : disk,
        ),
      );
      expect(screen.getByText('/unknown').closest('tr')).toHaveTextContent(
        '70% · 70.0 GB / 100 GB',
      );
      expect(screen.getByText('/no-capacity').closest('tr')).toHaveTextContent('Usage unavailable');
      setValue(disks);
      expect(screen.getByText('/unknown').closest('tr')).toHaveTextContent('Usage unavailable');
      expect(screen.getByText('/critical').closest('tr')).toHaveTextContent('95%');
    });
  });
}
