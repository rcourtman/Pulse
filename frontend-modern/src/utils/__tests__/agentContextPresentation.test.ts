import { describe, expect, it } from 'vitest';
import { formatAgentResourceContextForClipboard } from '@/utils/agentContextPresentation';

describe('formatAgentResourceContextForClipboard', () => {
  it('formats bounded resource context sections with provenance and redactions', () => {
    const text = formatAgentResourceContextForClipboard({
      canonicalId: 'app-container:homeassistant',
      resourceType: 'app-container',
      resourceName: 'homeassistant',
      technology: 'docker',
      activeFindings: [],
      pendingApprovals: [],
      recentActions: [],
      generatedAt: '2026-05-06T14:00:00Z',
      contextSections: [
        {
          id: 'runtime',
          title: 'Runtime and Discovery',
          source: 'unified-resource',
          trustTier: 'runtime-observed',
          observedAt: '2026-05-06T13:59:00Z',
          generatedAt: '2026-05-06T14:00:00Z',
          facts: [
            {
              label: 'Ports',
              value: '8123:8123/tcp',
              source: 'unified-resource',
              trustTier: 'discovered',
            },
            {
              label: 'Storage path',
              value: 'redacted by policy',
              source: 'unified-resource',
              trustTier: 'runtime-observed',
              redacted: true,
            },
          ],
          redactions: [
            {
              field: 'resource identifiers',
              reason: 'canonical resource policy redacts path',
            },
          ],
        },
      ],
    });

    expect(text).toContain('# Pulse resource context: homeassistant');
    expect(text).toContain('Canonical ID: app-container:homeassistant');
    expect(text).toContain('## Runtime and Discovery');
    expect(text).toContain('- Ports: 8123:8123/tcp');
    expect(text).toContain('- Storage path: redacted by policy');
    expect(text).toContain('redacted=true');
    expect(text).toContain('Redaction: resource identifiers');
  });
});
