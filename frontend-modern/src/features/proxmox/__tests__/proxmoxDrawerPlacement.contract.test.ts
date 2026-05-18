import { describe, expect, it } from 'vitest';

import workloadsStateSource from '@/components/Workloads/useWorkloadsState.ts?raw';
import workloadPanelSource from '@/components/Workloads/WorkloadPanel.tsx?raw';
import workloadsTableSource from '@/components/Workloads/WorkloadsTable.tsx?raw';
import proxmoxNodesTableSource from '../ProxmoxNodesTable.tsx?raw';
import proxmoxPageSurfaceSource from '../ProxmoxPageSurface.tsx?raw';

describe('Proxmox drawer placement contract', () => {
  it('keeps host details owned by the Proxmox host table instead of the embedded guest table', () => {
    expect(workloadsStateSource).toContain("groupNodeDrawerMode?: 'inline' | 'disabled';");
    expect(workloadsStateSource).toContain(
      "groupNodeDrawerMode: () => props.groupNodeDrawerMode ?? 'inline',",
    );
    expect(workloadsTableSource).toContain(`| 'groupNodeDrawerMode'`);
    expect(workloadsTableSource).toContain('groupNodeDrawerMode={props.groupNodeDrawerMode}');
    expect(workloadPanelSource).toContain("props.groupNodeDrawerMode() === 'inline'");
    expect(workloadPanelSource).toContain(
      'onClick: canOpenNodeDrawer() ? handleGroupFocusToggle : undefined',
    );
    expect(proxmoxPageSurfaceSource).toContain('groupNodeDrawerMode="disabled"');
    expect(proxmoxNodesTableSource).toContain('NodeDrawer');
    expect(proxmoxNodesTableSource).toContain('data-inline-node-detail-for={node.id}');
  });
});
