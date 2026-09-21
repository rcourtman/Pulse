// Browser fixture: the production Alert Thresholds card list (the narrow-layout
// renderer) with two Proxmox hosts whose guests share display names. It exists
// to reproduce #2130: a short leading group header corrupting the virtual
// window's item-height estimate so a host's rows vanish while scrolling.
// Synthetic props only; no backend, API or WebSocket path.
import { render } from 'solid-js/web';

import { AlertResourceTableMobile } from '../src/components/Alerts/AlertResourceTableMobile';
import type { ResourceTableProps } from '../src/components/Alerts/ResourceTable';
import type { Resource } from '../src/features/alerts/thresholds/tableTypes';
import '../src/index.css';

const HOSTS = ['pve01.staging.tld', 'pve01.production.tld'];
const COLUMNS = [
  'CPU %',
  'Memory %',
  'Disk %',
  'Backup',
  'Snapshot',
  'Disk R MB/s',
  'Disk W MB/s',
  'Net In MB/s',
  'Net Out MB/s',
];

const grouped: Record<string, Resource[]> = {};
for (const [hostIndex, host] of HOSTS.entries()) {
  grouped[host] = Array.from({ length: 15 }, (_, index) => {
    // Both environments expose a guest named gateway01, exactly as reported.
    const name =
      index === 0
        ? 'gateway01'
        : `${hostIndex === 0 ? 'staging' : 'production'}-vm-${String(index).padStart(2, '0')}`;
    return {
      id: `${host}-${index}`,
      name,
      displayName: name,
      rawName: name,
      type: 'guest',
      resourceType: index % 3 === 0 ? 'VM' : 'Container',
      vmid: 100 + index,
      node: host,
      instance: host,
      status: 'running',
      hasOverride: false,
      disabled: false,
      disableConnectivity: false,
      thresholds: {},
      defaults: { cpu: 80, memory: 85, disk: 90 },
      backup: { enabled: true },
      snapshot: { enabled: true },
    } satisfies Resource;
  });
}

const noop = () => undefined;
const table: ResourceTableProps = {
  title: '',
  columns: COLUMNS,
  groupedResources: grouped,
  onEdit: noop,
  onSaveEdit: noop,
  onCancelEdit: noop,
  onRemoveOverride: noop,
  editingId: () => null,
  editingThresholds: () => ({}),
  setEditingThresholds: noop,
  formatMetricValue: (_metric, value) => (typeof value === 'number' ? `${value}%` : '—'),
  hasActiveAlert: () => false,
  editingNote: () => '',
  setEditingNote: noop,
};

render(
  () => (
    <div
      id="thresholds-scroll"
      style={{
        height: '600px',
        overflow: 'auto',
        border: '1px solid #cbd5e1',
        padding: '12px',
      }}
    >
      <AlertResourceTableMobile
        table={table}
        hasRows={() => true}
        hasCustomGlobalDefaults={() => false}
        setActiveMetricInput={noop}
      />
    </div>
  ),
  document.getElementById('root')!,
);
