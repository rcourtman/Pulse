import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@solidjs/testing-library';
import type { WorkloadGuest } from '@/types/workloads';
import type { Memory, Disk, GuestNetworkInterface } from '@/types/api';

// ── Mocks ──────────────────────────────────────────────────────────────

const mockNavigate = vi.fn();

vi.mock('@solidjs/router', () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock('./DiskList', () => ({
  DiskList: (props: { disks: Disk[] }) => (
    <div data-testid="disk-list">DiskList({props.disks.length} disks)</div>
  ),
}));

vi.mock('../Discovery/DiscoveryTab', () => ({
  DiscoveryTab: (props: {
    resourceType: string;
    agentId?: string;
    resourceId: string;
    hostname: string;
  }) => (
    <div data-testid="discovery-tab">
      <span data-testid="disc-resource-type">{props.resourceType}</span>
      <span data-testid="disc-agent-id">{props.agentId}</span>
      <span data-testid="disc-resource-id">{props.resourceId}</span>
    </div>
  ),
}));

vi.mock('@/components/shared/WebInterfaceUrlField', () => ({
  WebInterfaceUrlField: (props: {
    metadataKind: string;
    metadataId: string;
    targetLabel: string;
  }) => (
    <div data-testid="web-interface-url-field">
      <span data-testid="url-kind">{props.metadataKind}</span>
      <span data-testid="url-id">{props.metadataId}</span>
      <span data-testid="url-label">{props.targetLabel}</span>
    </div>
  ),
}));

// After mocks, import the component under test
import { GuestDrawer } from './GuestDrawer';

// ── Helpers ────────────────────────────────────────────────────────────

/** Build a minimal WorkloadGuest with required fields, overridable via partial. */
function makeGuest(overrides: Partial<WorkloadGuest> = {}): WorkloadGuest {
  return {
    id: 'inst1-node1-100',
    vmid: 100,
    name: 'test-vm',
    node: 'node1',
    instance: 'inst1',
    status: 'running',
    type: 'qemu',
    cpu: 0.25,
    cpus: 4,
    memory: { total: 4294967296, used: 2147483648, free: 2147483648, usage: 0.5 },
    disk: { total: 10737418240, used: 5368709120, free: 5368709120, usage: 0.5 },
    networkIn: 1000,
    networkOut: 2000,
    diskRead: 500,
    diskWrite: 600,
    uptime: 86400,
    template: false,
    lastBackup: 0,
    tags: null,
    lock: '',
    lastSeen: new Date().toISOString(),
    ...overrides,
  } as WorkloadGuest;
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.useRealTimers();
});

// ── Tests ──────────────────────────────────────────────────────────────

describe('GuestDrawer', () => {
  // ── Tabs ──

  describe('tab switching', () => {
    it('renders Overview and Discovery tabs', () => {
      render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);
      expect(screen.getByText('Overview')).toBeInTheDocument();
      expect(screen.getByText('Discovery')).toBeInTheDocument();
    });

    it('starts on the Overview tab (discovery content hidden)', () => {
      const { container } = render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);
      const panels = container.querySelectorAll('[style*="overflow-anchor"]');
      expect(panels[0]).not.toHaveClass('hidden');
      expect(panels[1]).toHaveClass('hidden');
    });

    it('switches to Discovery tab on click', async () => {
      const { container } = render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);
      await fireEvent.click(screen.getByText('Discovery'));
      const panels = container.querySelectorAll('[style*="overflow-anchor"]');
      expect(panels[0]).toHaveClass('hidden');
      expect(panels[1]).not.toHaveClass('hidden');
    });

    it('switches back to Overview tab', async () => {
      const { container } = render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);
      await fireEvent.click(screen.getByText('Discovery'));
      await fireEvent.click(screen.getByText('Overview'));
      const panels = container.querySelectorAll('[style*="overflow-anchor"]');
      expect(panels[0]).not.toHaveClass('hidden');
      expect(panels[1]).toHaveClass('hidden');
    });
  });

  // ── Infrastructure navigation ──

  describe('infrastructure link', () => {
    it('navigates to proxmox infrastructure for a VM', async () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ node: 'pve1', type: 'qemu' })} onClose={vi.fn()} />
      ));
      await fireEvent.click(screen.getByText('Open related infrastructure'));
      expect(mockNavigate).toHaveBeenCalledTimes(1);
      const path = mockNavigate.mock.calls[0][0] as string;
      expect(path).toContain('infrastructure');
      expect(path).toContain('proxmox');
    });

    it('navigates to docker infrastructure for a docker workload', async () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ workloadType: 'docker', contextLabel: 'my-host' })}
          onClose={vi.fn()}
        />
      ));
      await fireEvent.click(screen.getByText('Open related infrastructure'));
      const path = mockNavigate.mock.calls[0][0] as string;
      expect(path).toContain('source=docker');
    });

    it('navigates to kubernetes infrastructure for a k8s workload', async () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ workloadType: 'k8s', contextLabel: 'my-cluster' })}
          onClose={vi.fn()}
        />
      ));
      await fireEvent.click(screen.getByText('Open related infrastructure'));
      const path = mockNavigate.mock.calls[0][0] as string;
      expect(path).toContain('kubernetes');
    });
  });

  // ── System card ──

  describe('System card', () => {
    it('displays CPU count', () => {
      render(() => <GuestDrawer guest={makeGuest({ cpus: 8 })} onClose={vi.fn()} />);
      expect(screen.getByText('CPUs')).toBeInTheDocument();
      expect(screen.getByText('8')).toBeInTheDocument();
    });

    it('displays uptime when > 0', () => {
      render(() => <GuestDrawer guest={makeGuest({ uptime: 3600 })} onClose={vi.fn()} />);
      expect(screen.getByText('Uptime')).toBeInTheDocument();
    });

    it('hides uptime when 0', () => {
      render(() => <GuestDrawer guest={makeGuest({ uptime: 0 })} onClose={vi.fn()} />);
      expect(screen.queryByText('Uptime')).not.toBeInTheDocument();
    });

    it('displays node name', () => {
      render(() => <GuestDrawer guest={makeGuest({ node: 'pve-prod-01' })} onClose={vi.fn()} />);
      expect(screen.getByText('Node')).toBeInTheDocument();
      // Node name appears in both overview and discovery mock; use getAllByText
      const nodes = screen.getAllByText('pve-prod-01');
      expect(nodes.length).toBeGreaterThanOrEqual(1);
    });
  });

  // ── Agent info ──

  describe('Agent info', () => {
    it('shows QEMU agent info for VMs', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ agentVersion: '5.2.0', type: 'qemu' })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('Agent')).toBeInTheDocument();
      expect(screen.getByText('QEMU 5.2.0')).toBeInTheDocument();
    });

    it('shows plain agent version for containers', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ agentVersion: '1.0.0', type: 'lxc' })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('Agent')).toBeInTheDocument();
      expect(screen.getByText('1.0.0')).toBeInTheDocument();
    });

    it('hides agent section when no agentVersion', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ agentVersion: undefined })} onClose={vi.fn()} />
      ));
      expect(screen.queryByText('Agent')).not.toBeInTheDocument();
    });
  });

  // ── Guest Info card (OS + IPs) ──

  describe('Guest Info card', () => {
    it('shows OS name and version', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ osName: 'Ubuntu', osVersion: '22.04' })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByText('Guest Info')).toBeInTheDocument();
      expect(screen.getByText('Ubuntu')).toBeInTheDocument();
      expect(screen.getByText('22.04')).toBeInTheDocument();
    });

    it('shows OS name only when version is missing', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ osName: 'Debian', osVersion: undefined })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByText('Debian')).toBeInTheDocument();
    });

    it('shows OS version only when name is missing', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ osName: undefined, osVersion: '11.0' })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByText('Guest Info')).toBeInTheDocument();
      expect(screen.getByText('11.0')).toBeInTheDocument();
    });

    it('shows IP addresses', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ ipAddresses: ['192.168.1.10', '10.0.0.5'] })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByText('192.168.1.10')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.5')).toBeInTheDocument();
    });

    it('hides Guest Info card when no OS info and no IPs', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ osName: undefined, osVersion: undefined, ipAddresses: [] })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.queryByText('Guest Info')).not.toBeInTheDocument();
    });
  });

  // ── Memory card ──

  describe('Memory card', () => {
    it('shows balloon info when different from total', () => {
      const memory: Memory = {
        total: 4294967296,
        used: 2147483648,
        free: 2147483648,
        usage: 0.5,
        balloon: 2147483648,
      };
      render(() => <GuestDrawer guest={makeGuest({ memory })} onClose={vi.fn()} />);
      expect(screen.getByText('Memory')).toBeInTheDocument();
      expect(screen.getByText(/Balloon/)).toBeInTheDocument();
    });

    it('shows swap info when swap is present', () => {
      const memory: Memory = {
        total: 4294967296,
        used: 2147483648,
        free: 2147483648,
        usage: 0.5,
        swapTotal: 2147483648,
        swapUsed: 1073741824,
      };
      render(() => <GuestDrawer guest={makeGuest({ memory })} onClose={vi.fn()} />);
      expect(screen.getByText(/Swap/)).toBeInTheDocument();
    });

    it('hides Memory card when no balloon and no swap', () => {
      const memory: Memory = {
        total: 4294967296,
        used: 2147483648,
        free: 2147483648,
        usage: 0.5,
      };
      render(() => <GuestDrawer guest={makeGuest({ memory })} onClose={vi.fn()} />);
      const memoryHeaders = screen.queryAllByText('Memory');
      expect(memoryHeaders).toHaveLength(0);
    });

    it('hides balloon when it equals total', () => {
      const memory: Memory = {
        total: 4294967296,
        used: 2147483648,
        free: 2147483648,
        usage: 0.5,
        balloon: 4294967296,
      };
      render(() => <GuestDrawer guest={makeGuest({ memory })} onClose={vi.fn()} />);
      expect(screen.queryByText(/Balloon/)).not.toBeInTheDocument();
    });
  });

  // ── Backup card ──

  describe('Backup card', () => {
    beforeEach(() => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date('2026-03-02T12:00:00Z'));
    });

    it('shows "Today" for a backup from today', () => {
      const now = new Date('2026-03-02T10:00:00Z').getTime();
      render(() => <GuestDrawer guest={makeGuest({ lastBackup: now })} onClose={vi.fn()} />);
      expect(screen.getByText('Backup')).toBeInTheDocument();
      expect(screen.getByText('Today')).toBeInTheDocument();
    });

    it('shows "Yesterday" for a 1-day-old backup', () => {
      const yesterday = new Date('2026-03-01T12:00:00Z').getTime();
      render(() => <GuestDrawer guest={makeGuest({ lastBackup: yesterday })} onClose={vi.fn()} />);
      expect(screen.getByText('Yesterday')).toBeInTheDocument();
    });

    it('shows "Xd ago" for older backups', () => {
      const fiveDaysAgo = new Date('2026-02-25T12:00:00Z').getTime();
      render(() => (
        <GuestDrawer guest={makeGuest({ lastBackup: fiveDaysAgo })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('5d ago')).toBeInTheDocument();
    });

    it('applies warning color for backups older than 7 days', () => {
      const tenDaysAgo = new Date('2026-02-20T12:00:00Z').getTime();
      const { container } = render(() => (
        <GuestDrawer guest={makeGuest({ lastBackup: tenDaysAgo })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('10d ago')).toBeInTheDocument();
      const ageEl = screen.getByText('10d ago');
      expect(ageEl.className).toContain('amber');
    });

    it('applies critical color for backups older than 30 days', () => {
      const fortyDaysAgo = new Date('2026-01-21T12:00:00Z').getTime();
      render(() => (
        <GuestDrawer guest={makeGuest({ lastBackup: fortyDaysAgo })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('40d ago')).toBeInTheDocument();
      const ageEl = screen.getByText('40d ago');
      expect(ageEl.className).toContain('red');
    });

    it('applies green color for recent backups', () => {
      const now = new Date('2026-03-02T10:00:00Z').getTime();
      render(() => <GuestDrawer guest={makeGuest({ lastBackup: now })} onClose={vi.fn()} />);
      const ageEl = screen.getByText('Today');
      expect(ageEl.className).toContain('green');
    });

    it('hides Backup card when lastBackup is 0 (falsy)', () => {
      render(() => <GuestDrawer guest={makeGuest({ lastBackup: 0 })} onClose={vi.fn()} />);
      expect(screen.queryByText('Backup')).not.toBeInTheDocument();
    });
  });

  // ── Tags card ──

  describe('Tags card', () => {
    it('renders tags from an array', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ tags: ['production', 'web'] })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('Tags')).toBeInTheDocument();
      expect(screen.getByText('production')).toBeInTheDocument();
      expect(screen.getByText('web')).toBeInTheDocument();
    });

    it('renders tags from a comma-separated string', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ tags: 'db,critical' as any })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('db')).toBeInTheDocument();
      expect(screen.getByText('critical')).toBeInTheDocument();
    });

    it('trims whitespace from tags', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ tags: ' spaced , padded ' as any })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('spaced')).toBeInTheDocument();
      expect(screen.getByText('padded')).toBeInTheDocument();
    });

    it('hides Tags card when tags is null', () => {
      render(() => <GuestDrawer guest={makeGuest({ tags: null })} onClose={vi.fn()} />);
      expect(screen.queryByText('Tags')).not.toBeInTheDocument();
    });

    it('hides Tags card when tags is empty array', () => {
      render(() => <GuestDrawer guest={makeGuest({ tags: [] })} onClose={vi.fn()} />);
      expect(screen.queryByText('Tags')).not.toBeInTheDocument();
    });
  });

  // ── Filesystems card ──

  describe('Filesystems card', () => {
    it('renders DiskList when disks are present', () => {
      const disks: Disk[] = [
        { total: 10737418240, used: 5368709120, free: 5368709120, usage: 0.5, mountpoint: '/' },
      ];
      render(() => <GuestDrawer guest={makeGuest({ disks })} onClose={vi.fn()} />);
      expect(screen.getByText('Filesystems')).toBeInTheDocument();
      expect(screen.getByTestId('disk-list')).toBeInTheDocument();
    });

    it('hides Filesystems card when disks is empty', () => {
      render(() => <GuestDrawer guest={makeGuest({ disks: [] })} onClose={vi.fn()} />);
      expect(screen.queryByText('Filesystems')).not.toBeInTheDocument();
    });

    it('hides Filesystems card when disks is undefined', () => {
      render(() => <GuestDrawer guest={makeGuest({ disks: undefined })} onClose={vi.fn()} />);
      expect(screen.queryByText('Filesystems')).not.toBeInTheDocument();
    });
  });

  // ── Network card ──

  describe('Network card', () => {
    it('renders network interfaces with name and traffic', () => {
      const networkInterfaces: GuestNetworkInterface[] = [
        {
          name: 'eth0',
          mac: 'aa:bb:cc:dd:ee:ff',
          rxBytes: 1024,
          txBytes: 2048,
          addresses: ['192.168.1.5'],
        },
      ];
      render(() => <GuestDrawer guest={makeGuest({ networkInterfaces })} onClose={vi.fn()} />);
      expect(screen.getByText('Network')).toBeInTheDocument();
      expect(screen.getByText('eth0')).toBeInTheDocument();
      expect(screen.getByText('aa:bb:cc:dd:ee:ff')).toBeInTheDocument();
      expect(screen.getByText('192.168.1.5')).toBeInTheDocument();
      expect(screen.getByText(/^RX /)).toBeInTheDocument();
      expect(screen.getByText(/^TX /)).toBeInTheDocument();
    });

    it('displays "interface" as fallback when name is missing', () => {
      const networkInterfaces: GuestNetworkInterface[] = [{ rxBytes: 100, txBytes: 200 }];
      render(() => <GuestDrawer guest={makeGuest({ networkInterfaces })} onClose={vi.fn()} />);
      expect(screen.getByText('interface')).toBeInTheDocument();
    });

    it('limits displayed interfaces to 4', () => {
      const networkInterfaces: GuestNetworkInterface[] = Array.from({ length: 6 }, (_, i) => ({
        name: `eth${i}`,
        rxBytes: 0,
        txBytes: 0,
      }));
      render(() => <GuestDrawer guest={makeGuest({ networkInterfaces })} onClose={vi.fn()} />);
      expect(screen.getByText('eth0')).toBeInTheDocument();
      expect(screen.getByText('eth3')).toBeInTheDocument();
      expect(screen.queryByText('eth4')).not.toBeInTheDocument();
      expect(screen.queryByText('eth5')).not.toBeInTheDocument();
    });

    it('hides Network card when no interfaces', () => {
      render(() => <GuestDrawer guest={makeGuest({ networkInterfaces: [] })} onClose={vi.fn()} />);
      expect(screen.queryByText('Network')).not.toBeInTheDocument();
    });

    it('hides traffic line when both rx and tx are 0', () => {
      const networkInterfaces: GuestNetworkInterface[] = [{ name: 'eth0', rxBytes: 0, txBytes: 0 }];
      render(() => <GuestDrawer guest={makeGuest({ networkInterfaces })} onClose={vi.fn()} />);
      expect(screen.getByText('eth0')).toBeInTheDocument();
      expect(screen.queryByText(/^RX /)).not.toBeInTheDocument();
    });
  });

  // ── WebInterfaceUrlField ──

  describe('WebInterfaceUrlField', () => {
    it('passes correct metadataId using guest.id', () => {
      render(() => <GuestDrawer guest={makeGuest({ id: 'my-guest-id' })} onClose={vi.fn()} />);
      expect(screen.getByTestId('url-id').textContent).toBe('my-guest-id');
    });

    it('builds fallback id from instance:node:vmid when id is empty', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ id: '', instance: 'pve', node: 'n1', vmid: 200 })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByTestId('url-id').textContent).toBe('pve:n1:200');
    });

    it('labels docker guests as "container"', () => {
      render(() => <GuestDrawer guest={makeGuest({ workloadType: 'docker' })} onClose={vi.fn()} />);
      expect(screen.getByTestId('url-label').textContent).toBe('container');
    });

    it('labels k8s guests as "workload"', () => {
      render(() => <GuestDrawer guest={makeGuest({ workloadType: 'k8s' })} onClose={vi.fn()} />);
      expect(screen.getByTestId('url-label').textContent).toBe('workload');
    });

    it('labels VM guests as "guest"', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ type: 'qemu', workloadType: undefined })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByTestId('url-label').textContent).toBe('guest');
    });

    it('labels LXC guests as "guest"', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ type: 'lxc', workloadType: undefined })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByTestId('url-label').textContent).toBe('guest');
    });
  });

  // ── Discovery tab integration ──

  describe('DiscoveryTab integration', () => {
    it('passes correct resourceType and agentId for VM', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ type: 'qemu', node: 'pve1', vmid: 101 })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByTestId('disc-resource-type').textContent).toBe('vm');
      expect(screen.getByTestId('disc-agent-id').textContent).toBe('pve1');
      expect(screen.getByTestId('disc-resource-id').textContent).toBe('101');
    });

    it('passes correct resourceType and agentId for docker', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({
            workloadType: 'docker',
            dockerHostId: 'dh-1',
            id: 'container-abc',
          })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByTestId('disc-resource-type').textContent).toBe('docker');
      expect(screen.getByTestId('disc-agent-id').textContent).toBe('dh-1');
      expect(screen.getByTestId('disc-resource-id').textContent).toBe('container-abc');
    });

    it('passes correct resourceType for k8s', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({
            workloadType: 'k8s',
            kubernetesAgentId: 'k8s-agent-1',
            id: 'k8s:ctx:pod:my-pod',
          })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByTestId('disc-resource-type').textContent).toBe('k8s');
      expect(screen.getByTestId('disc-agent-id').textContent).toBe('k8s-agent-1');
      expect(screen.getByTestId('disc-resource-id').textContent).toBe('my-pod');
    });

    it('passes system-container as resourceType for LXC', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ type: 'lxc', node: 'pve2', vmid: 200 })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByTestId('disc-resource-type').textContent).toBe('system-container');
      expect(screen.getByTestId('disc-agent-id').textContent).toBe('pve2');
      expect(screen.getByTestId('disc-resource-id').textContent).toBe('200');
    });
  });
});
