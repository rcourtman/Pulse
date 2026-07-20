import { describe, expect, it } from 'vitest';
import {
  buildResourceAssistantContextForTarget,
  type ResourceAssistantContextTarget,
} from '@/utils/resourceAssistantContextModel';
import type { AIChatContext } from '@/stores/aiChat';
import type { ResourceDiscoveryReadiness, ResourceDiscoveryTarget } from '@/types/resource';

// Minimal target factory matching the required fields of
// `ResourceAssistantContextTarget`. Optional facets are layered in per-test to
// drive specific branches of `buildResourceAssistantContextForTarget`.
const makeTarget = (
  overrides: Partial<ResourceAssistantContextTarget> = {},
): ResourceAssistantContextTarget => ({
  id: 'res-1',
  name: 'res-1',
  type: 'agent',
  source: 'resource-detail-drawer',
  ...overrides,
});

describe('buildResourceAssistantContextForTarget — branch coverage (branchcov2)', () => {
  describe('displayName = target.name || target.id', () => {
    it('uses target.name when it is a non-empty string', () => {
      const context = buildResourceAssistantContextForTarget(
        makeTarget({ id: 'id-7', name: 'web-prod-01' }),
      );
      expect(context.briefing?.title).toBe('web-prod-01');
      expect(context.handoffResources?.[0]?.name).toBe('web-prod-01');
    });

    it('falls back to target.id when name is an empty string', () => {
      const context = buildResourceAssistantContextForTarget(makeTarget({ id: 'id-7', name: '' }));
      expect(context.briefing?.title).toBe('id-7');
      expect(context.handoffResources?.[0]?.name).toBe('id-7');
    });

    it('falls back to target.id when name is undefined', () => {
      const context = buildResourceAssistantContextForTarget(
        makeTarget({ id: 'id-9', name: undefined }),
      );
      expect(context.briefing?.title).toBe('id-9');
    });
  });

  describe('subjectParts via compact() — type / status / technology routing', () => {
    it('joins all three populated parts with " / "', () => {
      const context = buildResourceAssistantContextForTarget(
        makeTarget({ type: 'vm', status: 'online', technology: 'vmware' }),
      );
      expect(context.briefing?.subject).toBe('vm / online / vmware');
    });

    it('keeps only the parts that are present (technology dropped)', () => {
      const context = buildResourceAssistantContextForTarget(
        makeTarget({ type: 'agent', status: 'offline', technology: undefined }),
      );
      expect(context.briefing?.subject).toBe('agent / offline');
    });

    it('keeps only the type when status and technology are absent', () => {
      const context = buildResourceAssistantContextForTarget(
        makeTarget({ type: 'storage', status: undefined, technology: undefined }),
      );
      expect(context.briefing?.subject).toBe('storage');
    });

    it('drops whitespace-only and empty-string values so the subject collapses to empty', () => {
      const context = buildResourceAssistantContextForTarget(
        makeTarget({ type: '   ', status: '', technology: ' \t ' }),
      );
      expect(context.briefing?.subject).toBe('');
    });
  });

  describe('statusLabel — readinessPresentation truthy vs null', () => {
    it('appends the readiness statusLabel when a discoveryReadiness is present', () => {
      // readiness present => readinessPresentation is non-null => concatenated arm.
      const context = buildResourceAssistantContextForTarget(
        makeTarget({
          discoveryReadiness: { state: 'fresh' } satisfies ResourceDiscoveryReadiness,
        }),
      );
      expect(context.briefing?.statusLabel).toBe('Read-only context attached · Discovery fresh');
    });

    it('appends the unknown statusLabel when discoveryTarget exists but readiness is absent', () => {
      // discoveryTarget truthy => Boolean(discoveryTarget) true => hasDiscoverySupport
      // true => readinessPresentation is the "unknown" shape even without readiness.
      const context = buildResourceAssistantContextForTarget(
        makeTarget({
          discoveryTarget: {
            resourceType: 'vm',
            agentId: 'agent-1',
            resourceId: 'res-1',
          } satisfies ResourceDiscoveryTarget,
        }),
      );
      expect(context.briefing?.statusLabel).toBe('Read-only context attached · Discovery unknown');
    });

    it('uses the plain statusLabel when neither readiness nor discoveryTarget are present', () => {
      // readiness absent AND discoveryTarget absent => readinessPresentation null.
      const context = buildResourceAssistantContextForTarget(makeTarget());
      expect(context.briefing?.statusLabel).toBe('Read-only context attached');
    });
  });

  describe('detailLines assembly — every conditional arm', () => {
    it('includes the Primary identity line only when primaryIdentity is truthy and differs from id', () => {
      // primaryIdentity && primaryIdentity !== target.id => truthy arm.
      const context = buildResourceAssistantContextForTarget(
        makeTarget({ id: 'r-1', primaryIdentity: 'web.example.com' }),
      );
      expect(context.briefing?.detailLines).toContain('Primary identity: web.example.com');
    });

    it('drops the Primary identity line when primaryIdentity equals id', () => {
      // primaryIdentity truthy BUT primaryIdentity === target.id => '' falsy arm.
      const context = buildResourceAssistantContextForTarget(
        makeTarget({ id: 'r-1', primaryIdentity: 'r-1' }),
      );
      expect(context.briefing?.detailLines).toEqual(['Resource ID: r-1']);
    });

    it('drops the Primary identity line when primaryIdentity is absent', () => {
      // primaryIdentity falsy => '' falsy arm.
      const context = buildResourceAssistantContextForTarget(
        makeTarget({ id: 'r-2', primaryIdentity: undefined }),
      );
      expect(context.briefing?.detailLines).toEqual(['Resource ID: r-2']);
    });

    it('includes the Parent line when parentName is truthy', () => {
      const context = buildResourceAssistantContextForTarget(
        makeTarget({ parentName: 'esxi-host-1' }),
      );
      expect(context.briefing?.detailLines).toContain('Parent: esxi-host-1');
      // parentName also surfaces as the handoff node.
      expect(context.handoffResources?.[0]?.node).toBe('esxi-host-1');
    });

    it('drops the Parent line and leaves node undefined when parentName is absent', () => {
      const context = buildResourceAssistantContextForTarget(makeTarget({ parentName: undefined }));
      expect(context.briefing?.detailLines?.some((l) => l.startsWith('Parent:'))).toBe(false);
      expect(context.handoffResources?.[0]?.node).toBeUndefined();
    });

    it('builds the Discovery line from discoveryTarget.resourceType:resourceId when present', () => {
      const context = buildResourceAssistantContextForTarget(
        makeTarget({
          discoveryTarget: {
            resourceType: 'system-container',
            agentId: 'agent-2',
            resourceId: 'ct-42',
          } satisfies ResourceDiscoveryTarget,
        }),
      );
      expect(context.briefing?.detailLines).toContain('Discovery: system-container:ct-42');
    });

    it('drops the Discovery line when discoveryTarget is absent', () => {
      const context = buildResourceAssistantContextForTarget(makeTarget({ discoveryTarget: null }));
      expect(context.briefing?.detailLines?.some((l) => l.startsWith('Discovery:'))).toBe(false);
    });

    it('includes the readiness briefing line when discoveryReadiness is present', () => {
      // readinessLine non-empty => included arm.
      const context = buildResourceAssistantContextForTarget(
        makeTarget({
          discoveryReadiness: {
            state: 'stale',
            serviceName: 'Home Assistant',
            factCount: 5,
            ageSeconds: 120,
          } satisfies ResourceDiscoveryReadiness,
        }),
      );
      expect(context.briefing?.detailLines).toContain(
        'Discovery data: Discovery stale, service Home Assistant, 5 facts',
      );
    });

    it('omits the readiness briefing line when discoveryReadiness is absent', () => {
      // readinessLine === '' => compact drops it.
      const context = buildResourceAssistantContextForTarget(
        makeTarget({ discoveryReadiness: null }),
      );
      expect(context.briefing?.detailLines).toEqual(['Resource ID: res-1']);
    });
  });

  describe('context + handoff shape', () => {
    it('echoes status into context.resourceStatus when status is provided', () => {
      const context = buildResourceAssistantContextForTarget(makeTarget({ status: 'maintenance' }));
      expect(context.context).toStrictEqual({
        source: 'resource-detail-drawer',
        resourceId: 'res-1',
        resourceType: 'agent',
        resourceStatus: 'maintenance',
      });
    });

    it('leaves context.resourceStatus undefined when status is absent', () => {
      const context = buildResourceAssistantContextForTarget(makeTarget({ status: undefined }));
      // Destructure to assert the literal key is absent (undefined), not omitted
      // from the returned object via toStrictEqual semantics.
      expect(context.context?.resourceStatus).toBeUndefined();
      expect(context.context).toStrictEqual({
        source: 'resource-detail-drawer',
        resourceId: 'res-1',
        resourceType: 'agent',
        resourceStatus: undefined,
      });
    });
  });

  describe('full-shape maximal target', () => {
    it('builds the complete AIChatContext when every optional facet is populated', () => {
      const target = makeTarget({
        id: 'vm-101',
        name: 'web-prod-01',
        type: 'vm',
        source: 'resource-detail-drawer',
        status: 'online',
        technology: 'vmware',
        parentName: 'esxi-host-1',
        primaryIdentity: 'web-prod-01.example.com',
        discoveryTarget: {
          resourceType: 'vm',
          agentId: 'agent-7',
          resourceId: 'vm-101',
        } satisfies ResourceDiscoveryTarget,
        discoveryReadiness: {
          state: 'stale',
          serviceName: 'Home Assistant',
          factCount: 5,
          ageSeconds: 120,
        } satisfies ResourceDiscoveryReadiness,
      });

      const expected: AIChatContext = {
        targetType: 'resource',
        targetId: 'vm-101',
        context: {
          source: 'resource-detail-drawer',
          resourceId: 'vm-101',
          resourceType: 'vm',
          resourceStatus: 'online',
        },
        briefing: {
          sourceLabel: 'Pulse resource context',
          title: 'web-prod-01',
          subject: 'vm / online / vmware',
          statusLabel: 'Read-only context attached · Discovery stale',
          detailLines: [
            'Resource ID: vm-101',
            'Primary identity: web-prod-01.example.com',
            'Parent: esxi-host-1',
            'Discovery: vm:vm-101',
            'Discovery data: Discovery stale, service Home Assistant, 5 facts',
          ],
          safetyNote: 'Approval required before any action.',
        },
        handoffResources: [{ id: 'vm-101', name: 'web-prod-01', type: 'vm', node: 'esxi-host-1' }],
        handoffMetadata: { kind: 'resource_context' },
        autonomousMode: false,
      };

      expect(buildResourceAssistantContextForTarget(target)).toStrictEqual(expected);
    });
  });
});
