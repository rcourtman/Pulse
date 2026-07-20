import { beforeEach, describe, expect, it } from 'vitest';
import { DEFAULT_LOCALE, loadLocaleCatalog, setActiveLocale } from '@/i18n';
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

describe('setupCompletionModel branch coverage (supplemental)', () => {
  beforeEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
  });

  describe('toConnectedSetupSystem', () => {
    it('falls back to the localized "Unknown" name when no display/host/name identity exists (en)', () => {
      // Drives the 3rd operand of the name chain (`getUnknownSetupSystemName()`) in
      // toConnectedSetupSystem: displayName, canonical display, hostname and
      // getPrimaryResourceIdentity all resolve to '' for an all-empty pbs resource.
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({ id: '', type: 'pbs', name: '', displayName: '', platformId: '' }),
      ]);

      expect(systems).toHaveLength(1);
      expect(systems[0]).toStrictEqual({
        id: '',
        name: 'Unknown',
        typeLabel: 'Proxmox Backup Server',
        host: '',
        connectionPath: 'api',
      });
    });

    it('returns the localized unknown name when a non-English catalog is active', async () => {
      // Exercises the i18n call inside getUnknownSetupSystemName(): in the es catalog
      // the unknown-name token is 'Desconocido', distinct from the English 'Unknown'.
      await loadLocaleCatalog('es');
      setActiveLocale('es');

      const systems = buildSetupCompletionConnectedSystems([
        makeResource({ id: '', type: 'pmg', name: '', displayName: '', platformId: '' }),
      ]);

      expect(systems).toHaveLength(1);
      expect(systems[0]!.name).toBe('Desconocido');
      expect(systems[0]!.typeLabel).toBe('Proxmox Mail Gateway');
      expect(systems[0]!.connectionPath).toBe('api');
    });

    it('routes an agent-facet agent resource onto the agent path deriving id from the agent facet', () => {
      // connectionPath ternary: isApiConnectedSetupResource false, isAgentConnectedSetupResource true.
      // id chain: platformId empty -> getActionableAgentIdFromResource -> agent facet id.
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'host-1',
          type: 'agent',
          platformType: 'agent',
          name: 'box',
          platformId: '',
          agent: { agentId: 'agent-99' },
        }),
      ]);

      expect(systems).toHaveLength(1);
      expect(systems[0]!.connectionPath).toBe('agent');
      expect(systems[0]!.id).toBe('agent-99');
    });

    it('drops an agent resource whose connectionPath resolves to null (no agent facet, no api platform)', () => {
      // connectionPath ternary third arm: neither api nor agent -> null -> early return null.
      const orphan = makeResource({
        id: 'lonely-1',
        type: 'agent',
        platformType: 'agent',
        name: 'lone',
      });
      expect(buildSetupCompletionConnectedSystems([orphan])).toEqual([]);
    });

    it('drops a non-infrastructure resource type via the isSetupCompletionInfrastructureResource guard', () => {
      // Early `if (!isSetupCompletionInfrastructureResource(resource)) return null` for type 'storage'.
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({ id: 'store-1', type: 'storage', name: 'shelf' }),
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
  });

  describe('getSetupCompletionPlatformLabel (observed via ConnectedSetupSystem.typeLabel)', () => {
    it('returns null (observed as the "Agent" typeLabel) when platformType is absent from the manifest', () => {
      // `if (!manifestPlatform) return null` arm; getConnectedSetupSystemTypeLabel falls back to 'Agent'.
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
    });

    it('returns null (observed as the "Agent" typeLabel) when platformType is the empty string', () => {
      // getSetupCompletionPlatformKey returns resource.platformType || null -> '' -> null key -> no manifest.
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

    it('returns the last display token for a multi-token platform (vmware-vsphere)', () => {
      // `displayTokens[displayTokens.length - 1]` truthy arm: ['vSphere', 'VMware vSphere'] -> last.
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'vc-1',
          type: 'agent',
          platformType: 'vmware-vsphere',
          name: 'vcsa',
          displayName: 'VC',
          platformId: 'vc-1',
          sourceType: 'api',
        }),
      ]);

      expect(systems[0]!.typeLabel).toBe('VMware vSphere');
    });

    it('returns the sole display token for a single-token platform (truenas)', () => {
      // Single-element displayTokens: ['TrueNAS'] -> displayTokens[0] is the last and only token.
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'tn-1',
          type: 'agent',
          platformType: 'truenas',
          name: 'tn',
          displayName: 'TN',
          platformId: 'tn-1',
          sourceType: 'api',
          identity: { hostname: 'tn-h' },
        }),
      ]);

      expect(systems[0]!.typeLabel).toBe('TrueNAS');
    });

    it('maps the pbs resource type to the proxmox-pbs manifest key regardless of platformType', () => {
      // getSetupCompletionPlatformKey hardcodes 'proxmox-pbs' for type pbs (ignoring platformType).
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'pbs-1',
          type: 'pbs',
          name: 'pbs',
          displayName: 'PBS',
          platformId: 'pbs-1',
          platformType: 'agent',
        }),
      ]);

      expect(systems[0]!.typeLabel).toBe('Proxmox Backup Server');
    });
  });

  describe('buildSetupCompletionConnectedSystems', () => {
    it('keys entries by the resolved system name when platformId and system id are both empty', () => {
      // Exercises the 3rd operand of `key = resource.platformId || nextSystem.id || nextSystem.name`:
      // two all-empty-identity pbs resources with different resolved names must NOT collide.
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({ id: '', type: 'pbs', name: '', displayName: '', platformId: '' }),
        makeResource({
          id: '',
          type: 'pbs',
          name: 'realbox',
          displayName: 'Real Box',
          platformId: '',
        }),
      ]);

      expect(systems).toHaveLength(2);
      expect(namesOf(systems)).toEqual(['Real Box', 'Unknown']);
    });

    it('does not downgrade an api entry when a later agent entry shares its key', () => {
      // Merge guard false arms: existing.connectionPath === 'agent' is false (it is 'api'),
      // !existing.host is false (host present), isUnknownSetupSystemName(existing.name) is false.
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'vc-a',
          type: 'agent',
          platformType: 'vmware-vsphere',
          name: 'vcsa',
          displayName: 'VC Box',
          platformId: 'shared',
          sourceType: 'api',
          identity: { hostname: 'vc-host' },
        }),
        makeResource({
          id: 'ag-b',
          type: 'agent',
          platformType: 'agent',
          name: 'Tower',
          displayName: 'Tower',
          platformId: 'shared',
          agent: { agentId: 'ag-b' },
        }),
      ]);

      expect(systems).toHaveLength(1);
      expect(systems[0]).toStrictEqual({
        id: 'shared',
        name: 'VC Box',
        typeLabel: 'VMware vSphere',
        host: 'vc-host',
        connectionPath: 'api',
      });
    });

    it('does not replace a known existing name with a later "Unknown" same-key entry', () => {
      // Name-replace guard false arm: isUnknownSetupSystemName(existing.name) is false because
      // 'Real One' is not the unknown token, so existing.name is retained.
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'pbs-a',
          type: 'pbs',
          name: 'real1',
          displayName: 'Real One',
          platformId: 'shared3',
        }),
        makeResource({
          id: 'pbs-b',
          type: 'pbs',
          name: 'unk',
          displayName: 'Unknown',
          platformId: 'shared3',
        }),
      ]);

      expect(systems).toHaveLength(1);
      expect(systems[0]!.name).toBe('Real One');
      expect(systems[0]!.connectionPath).toBe('api');
    });

    it('treats the localized "Desconocido" name as unknown under the es catalog and replaces it', async () => {
      // isUnknownSetupSystemName second || arm (`name === getUnknownSetupSystemName()`): under es
      // the unknown token is 'Desconocido', so an existing 'Desconocido' entry is replaced.
      await loadLocaleCatalog('es');
      setActiveLocale('es');

      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'pbs-d',
          type: 'pbs',
          name: 'd1',
          displayName: 'Desconocido',
          platformId: 'shared4',
        }),
        makeResource({
          id: 'pbs-e',
          type: 'pbs',
          name: 'r1',
          displayName: 'Real Deal',
          platformId: 'shared4',
        }),
      ]);

      expect(systems).toHaveLength(1);
      expect(systems[0]!.name).toBe('Real Deal');
    });

    it('does not treat "Desconocido" as unknown under the en catalog (first || arm only)', () => {
      // Counterpart to the es case: under en, getUnknownSetupSystemName() === 'Unknown', so
      // 'Desconocido' is NOT unknown and the existing name is retained.
      const systems = buildSetupCompletionConnectedSystems([
        makeResource({
          id: 'pbs-d',
          type: 'pbs',
          name: 'd1',
          displayName: 'Desconocido',
          platformId: 'shared4',
        }),
        makeResource({
          id: 'pbs-e',
          type: 'pbs',
          name: 'r1',
          displayName: 'Real Deal',
          platformId: 'shared4',
        }),
      ]);

      expect(systems).toHaveLength(1);
      expect(systems[0]!.name).toBe('Desconocido');
    });
  });
});
