import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { buildVmwarePageModel } from '../vmwarePageModel';

// The comparators and rank helpers under test (vmwareDatastoreDisplayName,
// compareVmwareDatastores, vmwareNetworkStatusRank, compareVmwareNetworks,
// vmwareVirtualMachineHostKey, vmwareVirtualMachineStatusRank,
// compareVmwareVirtualMachines) are module-private; they are exercised
// exclusively through buildVmwarePageModel's sorted datastores / networks /
// vms slices, so every assertion goes through that public entry point.
const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'lab',
  platformType: 'vmware-vsphere',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...resource,
});

describe('vmwarePageModel coverage', () => {
  describe('compareVmwareDatastores', () => {
    it('ranks datastores inaccessible < maintenance < attention < unknown < accessible', () => {
      // Names are intentionally reversed from rank order so that a name-based
      // sort would produce the opposite sequence, proving rank dominates.
      const inaccessible = makeResource({
        id: 'ds-inaccessible',
        type: 'storage',
        name: 'z-ds',
        storage: { topology: 'datastore' },
        vmware: { entityType: 'datastore', datastoreAccessible: false },
      });
      const maintenance = makeResource({
        id: 'ds-maintenance',
        type: 'storage',
        name: 'y-ds',
        storage: { topology: 'datastore' },
        vmware: { entityType: 'datastore', datastoreAccessible: true, maintenanceMode: 'in_maintenance' },
      });
      const attention = makeResource({
        id: 'ds-attention',
        type: 'storage',
        name: 'x-ds',
        storage: { topology: 'datastore' },
        vmware: { entityType: 'datastore', datastoreAccessible: true, overallStatus: 'red' },
      });
      const unknown = makeResource({
        id: 'ds-unknown',
        type: 'storage',
        name: 'w-ds',
        status: 'unknown',
        storage: { topology: 'datastore' },
        vmware: { entityType: 'datastore' },
      });
      const accessible = makeResource({
        id: 'ds-accessible',
        type: 'storage',
        name: 'v-ds',
        storage: { topology: 'datastore' },
        vmware: { entityType: 'datastore', datastoreAccessible: true },
      });

      const { datastores } = buildVmwarePageModel([
        accessible,
        unknown,
        attention,
        maintenance,
        inaccessible,
      ]);

      expect(datastores.map((d) => d.id)).toEqual([
        'ds-inaccessible',
        'ds-maintenance',
        'ds-attention',
        'ds-unknown',
        'ds-accessible',
      ]);
    });

    it('tie-breaks equal-rank datastores by display name with displayName > name > id precedence', () => {
      // All three are 'accessible' (same rank), forcing the displayName
      // tie-breaker. Each exercises a different fallback branch:
      //   - ds-by-display: displayName present -> should win over its name
      //   - ds-by-name:    empty displayName -> falls back to name
      //   - charlie:       empty displayName and name -> falls back to id
      const byDisplay = makeResource({
        id: 'ds-by-display',
        type: 'storage',
        displayName: 'alpha',
        name: 'zzz-ignored',
        storage: { topology: 'datastore' },
        vmware: { entityType: 'datastore', datastoreAccessible: true },
      });
      const byName = makeResource({
        id: 'ds-by-name',
        type: 'storage',
        displayName: '',
        name: 'bravo',
        storage: { topology: 'datastore' },
        vmware: { entityType: 'datastore', datastoreAccessible: true },
      });
      const byId = makeResource({
        id: 'charlie',
        type: 'storage',
        displayName: '',
        name: '',
        storage: { topology: 'datastore' },
        vmware: { entityType: 'datastore', datastoreAccessible: true },
      });

      const { datastores } = buildVmwarePageModel([byId, byName, byDisplay]);

      // alpha < bravo < charlie; if ds-by-display used its name ('zzz-ignored')
      // it would sort last, so its first position proves displayName precedence.
      expect(datastores.map((d) => d.id)).toEqual(['ds-by-display', 'ds-by-name', 'charlie']);
    });
  });

  describe('vmwareNetworkStatusRank / compareVmwareNetworks', () => {
    it('ranks networks attention < unknown < healthy', () => {
      // Names reversed from rank order to prove rank dominates the tie-break.
      const attention = makeResource({
        id: 'net-attention',
        type: 'network',
        name: 'z-net',
        vmware: { entityType: 'network', activeAlarmCount: 1 },
      });
      const unknown = makeResource({
        id: 'net-unknown',
        type: 'network',
        name: 'y-net',
        status: 'unknown',
        vmware: { entityType: 'network' },
      });
      const healthy = makeResource({
        id: 'net-healthy',
        type: 'network',
        name: 'x-net',
        vmware: { entityType: 'network', overallStatus: 'green' },
      });

      const { networks } = buildVmwarePageModel([healthy, unknown, attention]);

      expect(networks.map((n) => n.id)).toEqual([
        'net-attention',
        'net-unknown',
        'net-healthy',
      ]);
    });

    it('tie-breaks equal-rank networks by display name', () => {
      const alpha = makeResource({
        id: 'net-alpha',
        type: 'network',
        displayName: 'alpha',
        vmware: { entityType: 'network', overallStatus: 'green' },
      });
      const bravo = makeResource({
        id: 'net-bravo',
        type: 'network',
        displayName: 'bravo',
        vmware: { entityType: 'network', overallStatus: 'green' },
      });

      const { networks } = buildVmwarePageModel([bravo, alpha]);

      expect(networks.map((n) => n.id)).toEqual(['net-alpha', 'net-bravo']);
    });
  });

  describe('vmwareVirtualMachineHostKey', () => {
    it('groups VMs by host key using runtimeHostName > parentName > unknown and orders hosts alphabetically', () => {
      // vm-both has both runtimeHostName and parentName; it must group by the
      // runtimeHostName ('host-alpha') — proven by sorting ahead of host-beta.
      const both = makeResource({
        id: 'vm-both',
        type: 'vm',
        parentName: 'host-zeta',
        vmware: { runtimeHostName: 'host-alpha' },
      });
      const byParent = makeResource({
        id: 'vm-parent',
        type: 'vm',
        parentName: 'host-beta',
      });
      const byRuntime = makeResource({
        id: 'vm-runtime',
        type: 'vm',
        vmware: { runtimeHostName: 'host-zeta' },
      });
      const byFallback = makeResource({
        id: 'vm-fallback',
        type: 'vm',
      });

      const { vms } = buildVmwarePageModel([byFallback, byRuntime, byParent, both]);

      // host-alpha < host-beta < host-zeta < unknown
      expect(vms.map((v) => v.id)).toEqual([
        'vm-both',
        'vm-parent',
        'vm-runtime',
        'vm-fallback',
      ]);
    });
  });

  describe('vmwareVirtualMachineStatusRank / compareVmwareVirtualMachines', () => {
    it('ranks same-host VMs attention < suspended < powered-off < unknown < powered-on', () => {
      const host = { runtimeHostName: 'host-shared' };
      // Names reversed from rank order so a name-based sort would invert this.
      const attention = makeResource({
        id: 'vm-attention',
        type: 'vm',
        name: 'z-vm',
        vmware: { ...host, overallStatus: 'red' },
      });
      const suspended = makeResource({
        id: 'vm-suspended',
        type: 'vm',
        name: 'y-vm',
        vmware: { ...host, powerState: 'suspended' },
      });
      const poweredOff = makeResource({
        id: 'vm-powered-off',
        type: 'vm',
        name: 'x-vm',
        vmware: { ...host, powerState: 'poweredOff' },
      });
      const unknown = makeResource({
        id: 'vm-unknown',
        type: 'vm',
        name: 'w-vm',
        status: 'pending' as unknown as Resource['status'],
        vmware: { ...host },
      });
      const poweredOn = makeResource({
        id: 'vm-powered-on',
        type: 'vm',
        name: 'v-vm',
        vmware: { ...host, powerState: 'poweredOn' },
      });

      const { vms } = buildVmwarePageModel([
        poweredOn,
        unknown,
        poweredOff,
        suspended,
        attention,
      ]);

      expect(vms.map((v) => v.id)).toEqual([
        'vm-attention',
        'vm-suspended',
        'vm-powered-off',
        'vm-unknown',
        'vm-powered-on',
      ]);
    });

    it('tie-breaks same-host same-rank VMs by display name', () => {
      const alpha = makeResource({
        id: 'vm-alpha',
        type: 'vm',
        displayName: 'alpha',
        vmware: { runtimeHostName: 'host-shared', powerState: 'poweredOn' },
      });
      const bravo = makeResource({
        id: 'vm-bravo',
        type: 'vm',
        displayName: 'bravo',
        vmware: { runtimeHostName: 'host-shared', powerState: 'poweredOn' },
      });

      const { vms } = buildVmwarePageModel([bravo, alpha]);

      expect(vms.map((v) => v.id)).toEqual(['vm-alpha', 'vm-bravo']);
    });
  });
});
