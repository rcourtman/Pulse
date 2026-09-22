// Browser fixture: a Proxmox Backups PBS host whose Pulse agent is surfaced
// both as a PVE guest (with agent telemetry) and as a standalone source=pbs
// host row. Reproduces #1723: before the fix the two identity matches were
// treated as ambiguous, so the PBS row kept its service metrics target and the
// History tab showed "Collecting history" for a host that has history.
// Synthetic props only; the check script intercepts the metrics-history request.
import { render } from 'solid-js/web';

import { ProxmoxBackupServersTable } from '../src/features/proxmox/ProxmoxBackupServersTable';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const sharedAgent = { agentId: 'agent-proxback', hostname: 'proxback-vm' };

const pbs = {
  id: 'pbs-1',
  type: 'pbs',
  name: 'proxback',
  displayName: 'proxback',
  platformId: 'pbs-1',
  platformType: 'proxmox-pbs',
  sourceType: 'hybrid',
  sources: ['pbs'],
  status: 'online',
  lastSeen: Date.now(),
  cpu: { current: 4.7 },
  memory: { current: 20.3, total: 8000, used: 1624, free: 6376 },
  pbs: {
    instanceId: 'proxback',
    hostname: 'proxback-vm',
    version: '3.2.1',
    connectionHealth: 'healthy',
    datastores: [{ name: 'tank', total: 1000, used: 400, available: 600, usagePercent: 40 }],
  },
  // The PBS service target: correct for the service, but it has no host series.
  metricsTarget: { resourceType: 'agent', resourceId: 'pbs-1' },
  platformData: {
    sources: ['pbs'],
    pbs: { instanceId: 'proxback', hostname: 'proxback-vm', datastoreCount: 1 },
  },
} as unknown as Resource;

const guest = {
  id: 'vm-100',
  type: 'vm',
  name: 'proxback-vm',
  displayName: 'proxback-vm',
  platformId: 'proxmox:100',
  platformType: 'proxmox-pve',
  sourceType: 'hybrid',
  sources: ['proxmox', 'agent'],
  status: 'online',
  lastSeen: Date.now(),
  cpu: { current: 15.4 },
  memory: { current: 16.9, total: 8000, used: 1352, free: 6648 },
  agent: sharedAgent,
  metricsTarget: { resourceType: 'vm', resourceId: 'proxmox:100' },
  platformData: { sources: ['proxmox', 'agent'], agent: sharedAgent },
} as unknown as Resource;

const standalone = {
  id: 'agent-proxback',
  type: 'agent',
  name: 'proxback-vm',
  displayName: 'proxback-vm',
  platformId: 'agent-proxback',
  platformType: 'proxmox-pbs',
  sourceType: 'hybrid',
  sources: ['agent', 'pbs'],
  status: 'online',
  lastSeen: Date.now(),
  agent: sharedAgent,
  metricsTarget: { resourceType: 'agent', resourceId: 'agent-proxback' },
  platformData: { sources: ['agent', 'pbs'], agent: sharedAgent },
} as unknown as Resource;

render(
  () => <ProxmoxBackupServersTable servers={[pbs, guest, standalone]} />,
  document.getElementById('root') as HTMLElement,
);
