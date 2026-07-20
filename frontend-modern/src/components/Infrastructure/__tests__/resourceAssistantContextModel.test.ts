import { describe, expect, it } from 'vitest';
import { buildResourceAssistantContext } from '@/utils/resourceAssistantContextModel';
import type { Resource } from '@/types/resource';

const resource: Resource = {
  id: 'app-container:homeassistant',
  type: 'app-container',
  name: 'homeassistant',
  displayName: 'Home Assistant',
  platformId: 'homeassistant',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'online',
  technology: 'docker',
  parentName: 'ha-lxc',
  lastSeen: Date.now(),
  discoveryTarget: {
    resourceType: 'app-container',
    agentId: 'agent:pve-1',
    resourceId: 'homeassistant',
  },
  discoveryReadiness: {
    state: 'fresh',
    source: 'service-discovery',
    serviceName: 'Home Assistant',
    factCount: 5,
  },
};

describe('buildResourceAssistantContext', () => {
  it('creates a model-only resource handoff without prefilled prompt context', () => {
    const context = buildResourceAssistantContext(resource);

    expect(context.targetType).toBe('resource');
    expect(context.targetId).toBe('app-container:homeassistant');
    expect(context.autonomousMode).toBe(false);
    expect(context.handoffContext).toBeUndefined();
    expect(context.handoffResources).toEqual([
      {
        id: 'app-container:homeassistant',
        name: 'Home Assistant',
        type: 'app-container',
        node: 'ha-lxc',
      },
    ]);
    expect(context.handoffMetadata).toEqual({ kind: 'resource_context' });
    expect(context.briefing?.detailLines).toContain('Resource ID: app-container:homeassistant');
    expect(context.briefing?.detailLines).toContain('Parent: ha-lxc');
    expect(context.briefing?.detailLines).toContain('Discovery: app-container:homeassistant');
    expect(context.briefing?.detailLines).toContain(
      'Discovery data: Discovery fresh, service Home Assistant, 5 facts',
    );
    expect(context.briefing?.statusLabel).toBe('Read-only context attached · Discovery fresh');
  });
});
