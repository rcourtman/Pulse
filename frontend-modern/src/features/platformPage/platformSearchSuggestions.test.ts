import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { buildPlatformResourceSearchSuggestions } from './platformSearchSuggestions';

describe('buildPlatformResourceSearchSuggestions', () => {
  it('projects canonical resource identity without exposing arbitrary metadata', () => {
    const resource = {
      id: 'k8s:cluster-a:pod:api-7f9',
      type: 'pod',
      name: 'api-7f9',
      displayName: 'api',
      status: 'running',
      parentName: 'cluster-a',
      tags: ['production'],
      canonicalIdentity: {
        displayName: 'api-primary',
        aliases: ['api-service'],
        primaryId: 'pod/api-7f9',
      },
      metadata: { secretValue: 'must-not-leak' },
    } as unknown as Resource;

    const [suggestion] = buildPlatformResourceSearchSuggestions([resource]);

    expect(suggestion.label).toBe('api-primary');
    expect(suggestion.value).toBe('api-primary');
    expect(suggestion.description).toContain('cluster-a');
    expect(suggestion.keywords).toEqual(
      expect.arrayContaining(['k8s:cluster-a:pod:api-7f9', 'api-service', 'production']),
    );
    expect(suggestion.keywords).not.toContain('must-not-leak');
  });

  it('uses canonical ids as exact search values when display names collide', () => {
    const resources = ['cluster-a', 'cluster-b'].map(
      (cluster) =>
        ({
          id: `${cluster}:pod:api`,
          type: 'pod',
          name: 'api',
          displayName: 'api',
          status: 'running',
          parentName: cluster,
        }) as Resource,
    );

    expect(buildPlatformResourceSearchSuggestions(resources).map((item) => item.value)).toEqual([
      'cluster-a:pod:api',
      'cluster-b:pod:api',
    ]);
  });
});
