import { describe, expect, it } from 'vitest';

import workloadsStateSource from '@/components/Workloads/useWorkloadsState.ts?raw';
import workloadPanelSource from '@/components/Workloads/WorkloadPanel.tsx?raw';
import proxmoxNodesTableSource from '../ProxmoxNodesTable.tsx?raw';
import proxmoxPageSurfaceSource from '../ProxmoxPageSurface.tsx?raw';

describe('Proxmox drawer placement contract', () => {
  it('keeps host details owned by the Proxmox host table instead of the embedded guest table', () => {
    expect(workloadsStateSource).not.toContain('groupNodeDrawerMode');
    expect(workloadPanelSource).not.toContain('NodeDrawer');
    expect(proxmoxPageSurfaceSource).not.toContain('groupNodeDrawerMode');
    expect(proxmoxNodesTableSource).toContain('NodeDrawer');
    expect(proxmoxNodesTableSource).toContain('data-inline-node-detail-for={node.id}');
  });
});
