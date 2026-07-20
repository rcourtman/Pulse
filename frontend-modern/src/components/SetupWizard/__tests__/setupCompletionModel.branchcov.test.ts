import { beforeEach, describe, expect, it } from 'vitest';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import type { Resource } from '@/types/resource';
import {
  buildSetupCompletionConnectedSystems,
  type ConnectedSetupSystem,
} from '../setupCompletionModel';

type ResourceSeed = Partial<Resource> & Pick<Resource, 'id' | 'type'>;

const makeResource = (seed: ResourceSeed): Resource => ({
  name: '',
  displayName: '',
  platformId: '',
  platformType: 'agent',
  sourceType: 'agent',
  status: 'online',
  lastSeen: 0,
  ...seed,
});

const namesOf = (systems: readonly ConnectedSetupSystem[]): string[] => systems.map((s) => s.name);
const idsOf = (systems: readonly ConnectedSetupSystem[]): string[] => systems.map((s) => s.id);

describe('setupCompletionModel branch coverage', () => {
  beforeEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
  });

  describe('isSetupCompletionInfrastructureResource (via inclusion/exclusion)', () => {
    it('admits agent resources with an agent facet onto the agent connection path', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'agent-1',
          type: 'agent',
          name: 'Tower',
          agent: { agentId: 'agent-1' },
        }),
      ]);

      expect(systems).toHaveLength(1);
      expect(systems[0]!.connectionPath).toBe('agent');
      expect(systems[0]!.name).toBe('Tower');
      expect(systems[0]!.typeLabel).toBe('Agent');
      expect(systems[0]!.host).toBe('Tower');
    });

    it('admits pbs resources onto the api connection path regardless of agent facet', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'pbs-1',
          type: 'pbs',
          name: 'pbs-node',
          displayName: 'PBS Main',
          platformId: 'pbs-1',
        }),
      ]);

      expect(systems).toHaveLength(1);
      expect(systems[0]!.connectionPath).toBe('api');
      expect(systems[0]!.id).toBe('pbs-1');
      expect(systems[0]!.name).toBe('PBS Main');
      expect(systems[0]!.typeLabel).toBe('Proxmox Backup Server');
    });

    it('admits pmg resources onto the api connection path', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'pmg-1',
          type: 'pmg',
          name: 'pmg-node',
          displayName: 'PMG Main',
          platformId: 'pmg-1',
        }),
      ]);

      expect(systems).toHaveLength(1);
      expect(systems[0]!.connectionPath).toBe('api');
      expect(systems[0]!.typeLabel).toBe('Proxmox Mail Gateway');
    });

    it('drops non-infrastructure resource types (vm) entirely', () => {
      const vm = makeResource({ id: 'vm-1', type: 'vm', name: 'myvm' });
      expect(buildSetupCompletionConnectedSystems([vm])).toEqual([]);

      const mixed = buildSetupCompletionConnectedSystems([
        vm,
        makeResource({
          id: 'agent-1',
          type: 'agent',
          name: 'Tower',
          agent: { agentId: 'agent-1' },
        }),
      ]);
      expect(namesOf(mixed)).toEqual(['Tower']);
    });
  });

  describe('getSetupCompletionPlatformKey / getSetupCompletionPlatformLabel (typeLabel)', () => {
    it('maps pbs type to the proxmox-pbs manifest label', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'pbs-1',
          type: 'pbs',
          name: 'pbs',
          displayName: 'PBS Main',
          platformId: 'pbs-1',
          platformType: 'agent',
        }),
      ]);
      expect(systems[0]!.typeLabel).toBe('Proxmox Backup Server');
    });

    it('maps pmg type to the proxmox-pmg manifest label', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'pmg-1',
          type: 'pmg',
          name: 'pmg',
          displayName: 'PMG Main',
          platformId: 'pmg-1',
          platformType: 'agent',
        }),
      ]);
      expect(systems[0]!.typeLabel).toBe('Proxmox Mail Gateway');
    });

    it('maps a truenas agent (platformType truenas) to the truenas key and api path', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'tn-1',
          type: 'agent',
          platformType: 'truenas',
          name: 'tn',
          displayName: 'TrueNAS Main',
          platformId: 'tn-1',
          sourceType: 'api',
          identity: { hostname: 'tn-host' },
        }),
      ]);
      expect(systems[0]!.typeLabel).toBe('TrueNAS');
      expect(systems[0]!.connectionPath).toBe('api');
      expect(systems[0]!.host).toBe('tn-host');
    });

    it('passes a known platformType (vmware-vsphere) through to the manifest label', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'vc-1',
          type: 'agent',
          platformType: 'vmware-vsphere',
          name: 'vcsa',
          displayName: 'vCenter',
          platformId: 'vc-1',
          sourceType: 'api',
        }),
      ]);
      expect(systems[0]!.typeLabel).toBe('VMware vSphere');
      expect(systems[0]!.connectionPath).toBe('api');
    });

    it('falls back to the Agent typeLabel when platformType is absent from the manifest', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'gen-1',
          type: 'agent',
          platformType: 'generic',
          name: 'box',
          agent: { agentId: 'gen-1' },
        }),
      ]);
      expect(systems[0]!.typeLabel).toBe('Agent');
      expect(systems[0]!.connectionPath).toBe('agent');
    });

    it('falls back to the Agent typeLabel when platformType is empty (null key)', () => {
      const emptyPlatformType: Resource = {
        ...makeResource({
          id: 'emptypt-1',
          type: 'agent',
          name: 'box',
          agent: { agentId: 'emptypt-1' },
        }),
        platformType: '' as Resource['platformType'],
      };

      const systems = buildSetupCompletionConnectedSystems([emptyPlatformType]);
      expect(systems).toHaveLength(1);
      expect(systems[0]!.typeLabel).toBe('Agent');
    });
  });

  describe('getConnectedSetupSystemHost fallback chain', () => {
    it('uses the canonical hostname from identity when present (happy path)', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'h-1',
          type: 'agent',
          name: 'raw',
          platformId: 'h-1',
          agent: { agentId: 'h-1' },
          identity: { hostname: 'real-host.example' },
        }),
      ]);
      expect(systems[0]!.host).toBe('real-host.example');
    });

    it('falls back to platformData.proxmox.instance when no preferred hostname exists', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'pve-1',
          type: 'agent',
          platformType: 'agent',
          name: '',
          platformId: '',
          agent: { agentId: 'pve-1' },
          platformData: { proxmox: { instance: 'pve-node-1' } },
        }),
      ]);
      expect(systems[0]!.host).toBe('pve-node-1');
      expect(systems[0]!.id).toBe('pve-1');
      expect(systems[0]!.name).toBe('agent:pve-1');
    });

    it('falls back to platformData.truenas.hostname when no preferred hostname exists', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'tn-host-1',
          type: 'agent',
          platformType: 'truenas',
          name: '',
          platformId: '',
          platformData: { truenas: { hostname: 'tn-array-host' } },
        }),
      ]);
      expect(systems[0]!.host).toBe('tn-array-host');
      expect(systems[0]!.connectionPath).toBe('api');
    });

    it('falls back to platformData.vmware.hostname when no preferred hostname exists', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'vc-host-1',
          type: 'agent',
          platformType: 'vmware-vsphere',
          name: '',
          platformId: '',
          platformData: { vmware: { hostname: 'vc-array-host' } },
        }),
      ]);
      expect(systems[0]!.host).toBe('vc-array-host');
    });

    it('returns an empty host string when nothing is resolvable', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'empty-1',
          type: 'agent',
          platformType: 'agent',
          name: '',
          platformId: '',
          agent: { agentId: 'empty-1' },
        }),
      ]);
      expect(systems[0]!.host).toBe('');
      expect(systems[0]!.connectionPath).toBe('agent');
    });
  });

  describe('toConnectedSetupSystem (id/name/connectionPath/null arms)', () => {
    it('returns null for an agent resource with neither api nor agent connection evidence', () => {
      const orphan = makeResource({
        id: 'orphan-1',
        type: 'agent',
        platformType: 'agent',
        name: 'orphan',
      });
      expect(buildSetupCompletionConnectedSystems([orphan])).toEqual([]);
    });

    it('keeps a connected sibling while dropping the unconnected orphan', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'orphan-1',
          type: 'agent',
          platformType: 'agent',
          name: 'orphan',
        }),
        makeResource({
          id: 'agent-1',
          type: 'agent',
          platformType: 'agent',
          name: 'Tower',
          agent: { agentId: 'agent-1' },
        }),
      ]);
      expect(namesOf(systems)).toEqual(['Tower']);
    });

    it('resolves id from getActionableAgentIdFromResource when platformId is empty', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'res-id',
          type: 'agent',
          platformType: 'agent',
          name: '',
          platformId: '',
          agent: { agentId: 'agent-xyz' },
        }),
      ]);
      expect(systems[0]!.id).toBe('agent-xyz');
    });

    it('resolves id from resource.id when neither platformId nor agent id is set', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'tn-fallback',
          type: 'agent',
          platformType: 'truenas',
          name: '',
          platformId: '',
          platformData: { truenas: { hostname: 'tn-h' } },
        }),
      ]);
      expect(systems[0]!.id).toBe('tn-fallback');
    });

    it('prefers displayName for the system name', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'd-1',
          type: 'agent',
          platformType: 'agent',
          name: 'rawname',
          displayName: 'Pretty Name',
          agent: { agentId: 'd-1' },
        }),
      ]);
      expect(systems[0]!.name).toBe('Pretty Name');
    });

    it('falls back to resource.name when displayName is empty', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'n-1',
          type: 'agent',
          platformType: 'agent',
          name: 'myhost',
          agent: { agentId: 'n-1' },
        }),
      ]);
      expect(systems[0]!.name).toBe('myhost');
    });
  });

  describe('buildSetupCompletionConnectedSystems (dedupe, merge, sort)', () => {
    it('returns an empty array for an empty input list', () => {
      expect(buildSetupCompletionConnectedSystems([])).toEqual([]);
    });

    it('keeps the first entry when two api systems share a key and no merge rule fires', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'vc-a',
          type: 'agent',
          platformType: 'vmware-vsphere',
          name: 'vcsa-a',
          displayName: 'First VC',
          platformId: 'shared-vc',
          sourceType: 'api',
        }),
        makeResource({
          id: 'vc-b',
          type: 'agent',
          platformType: 'vmware-vsphere',
          name: 'vcsa-b',
          displayName: 'Second VC',
          platformId: 'shared-vc',
          sourceType: 'api',
        }),
      ]);
      expect(systems).toHaveLength(1);
      expect(systems[0]!.name).toBe('First VC');
      expect(systems[0]!.connectionPath).toBe('api');
    });

    it('upgrades an agent entry to api and replaces an Unknown name with a real one', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'a-unk',
          type: 'agent',
          platformType: 'agent',
          name: 'Unknown',
          platformId: 'shared-key',
          agent: { agentId: 'a-unk' },
        }),
        makeResource({
          id: 'b-api',
          type: 'agent',
          platformType: 'truenas',
          name: 'b-real',
          displayName: 'Real System',
          platformId: 'shared-key',
          sourceType: 'api',
          identity: { hostname: 'real-host' },
        }),
      ]);
      expect(systems).toHaveLength(1);
      expect(systems[0]!.connectionPath).toBe('api');
      expect(systems[0]!.typeLabel).toBe('TrueNAS');
      expect(systems[0]!.name).toBe('Real System');
      expect(systems[0]!.host).toBe('Unknown');
    });

    it('copies a missing host from a later same-key system onto an empty-host entry', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'a-empty',
          type: 'agent',
          platformType: 'agent',
          name: '',
          platformId: '',
          agent: { agentId: 'a-empty' },
        }),
        makeResource({
          id: 'b-vc',
          type: 'agent',
          platformType: 'vmware-vsphere',
          name: 'bvc',
          displayName: 'VC Box',
          platformId: 'a-empty',
          sourceType: 'api',
          identity: { hostname: 'vc-host' },
        }),
      ]);
      expect(systems).toHaveLength(1);
      expect(systems[0]!.host).toBe('vc-host');
      expect(systems[0]!.connectionPath).toBe('api');
      expect(systems[0]!.typeLabel).toBe('VMware vSphere');
    });

    it('sorts systems by name, then by id', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'c1',
          type: 'agent',
          platformType: 'agent',
          name: 'Charlie',
          platformId: 'c1',
          agent: { agentId: 'c1' },
        }),
        makeResource({
          id: 'a1',
          type: 'agent',
          platformType: 'agent',
          name: 'Alpha',
          platformId: 'a1',
          agent: { agentId: 'a1' },
        }),
        makeResource({
          id: 'b1',
          type: 'agent',
          platformType: 'agent',
          name: 'Bravo',
          platformId: 'b1',
          agent: { agentId: 'b1' },
        }),
      ]);
      expect(namesOf(systems)).toEqual(['Alpha', 'Bravo', 'Charlie']);
      expect(idsOf(systems)).toEqual(['a1', 'b1', 'c1']);
    });

    it('breaks a name tie by id', () => {
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 's-z',
          type: 'pbs',
          name: 'samebox',
          displayName: 'Same Name',
          platformId: 's-z',
        }),
        makeResource({
          id: 's-a',
          type: 'pbs',
          name: 'samebox2',
          displayName: 'Same Name',
          platformId: 's-a',
        }),
      ]);
      expect(namesOf(systems)).toEqual(['Same Name', 'Same Name']);
      expect(idsOf(systems)).toEqual(['s-a', 's-z']);
    });
  });
});
