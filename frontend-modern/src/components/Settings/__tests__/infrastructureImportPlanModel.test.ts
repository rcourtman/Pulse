import { describe, expect, it } from 'vitest';
import type { ProbeCandidate } from '@/api/connections';
import { getNodeModalDefaultFormData } from '@/utils/nodeModalPresentation';
import type { DiscoveredServer } from '../infrastructureSettingsModel';
import { buildNodeImportPlan } from '../infrastructureImportPlanModel';

describe('infrastructureImportPlanModel', () => {
  it('maps discovered Proxmox nodes to the monitored-system preview contract', () => {
    const discovered: DiscoveredServer = {
      type: 'pve',
      ip: '10.0.0.10',
      hostname: 'tower.local',
      port: 8006,
      version: 'Proxmox VE',
      release: '9.0',
    };
    const formData = {
      ...getNodeModalDefaultFormData('pve'),
      name: 'tower',
      host: 'https://tower.local:8006',
      authType: 'token',
      setupMode: 'auto',
    } as const;

    const plan = buildNodeImportPlan('pve', formData, {
      kind: 'discovery',
      server: discovered,
    });

    expect(plan).not.toBeNull();
    expect(plan?.source).toBe('proxmox');
    expect(plan?.sourceLabel).toBe('Proxmox VE');
    expect(plan?.endpoint).toBe('https://tower.local:8006');
    expect(plan?.detectedVersion).toBe('Proxmox VE 9.0');
    expect(plan?.previewRequest).toEqual({
      candidate: {
        source: 'proxmox',
        type: 'agent',
        name: 'tower',
        hostname: 'tower.local',
        host_url: 'https://tower.local:8006',
        resource_id: 'tower',
      },
    });
  });

  it('builds PBS probe import plans with explicit coverage and source identity', () => {
    const candidate: ProbeCandidate = {
      type: 'pbs',
      host: 'https://backup.local:8007',
      port: 8007,
      hints: {
        product: 'Proxmox Backup Server',
        version: '3.2',
      },
    };
    const formData = {
      ...getNodeModalDefaultFormData('pbs'),
      name: 'backup',
      host: 'https://backup.local:8007',
      authType: 'token',
      setupMode: 'auto',
      monitorVerifyJobs: false,
      monitorGarbageJobs: false,
    } as const;

    const plan = buildNodeImportPlan('pbs', formData, {
      kind: 'probe',
      candidate,
    });

    expect(plan).not.toBeNull();
    expect(plan?.source).toBe('pbs');
    expect(plan?.detectedVersion).toBe('Proxmox Backup Server 3.2');
    expect(plan?.coverageLabel).toBe('datastores, sync jobs, prune jobs');
    expect(plan?.previewRequest?.candidate).toMatchObject({
      source: 'pbs',
      type: 'pbs',
      name: 'backup',
      hostname: 'backup.local',
      host_url: 'https://backup.local:8007',
      resource_id: 'backup',
    });
  });
});
