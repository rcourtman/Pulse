import { describe, expect, it } from 'vitest';
import { LEGACY_REDIRECTS } from '../legacyRedirects';
import { buildLegacyRedirectTarget, mergeRedirectQueryParams } from '../navigation';
import { parseInfrastructureLinkSearch, parseWorkloadsLinkSearch } from '../resourceLinks';

describe('legacy route contracts', () => {
  it('keeps services redirect metadata while preserving incoming query params', () => {
    const target = buildLegacyRedirectTarget(
      LEGACY_REDIRECTS.services.destination,
      LEGACY_REDIRECTS.services.source,
    );
    const merged = mergeRedirectQueryParams(target, '?search=pmg&resource=pmg-main');

    const [, query = ''] = merged.split('?');
    const params = new URLSearchParams(query);
    expect(params.get('source')).toBe('pmg');
    expect(params.get('migrated')).toBe('1');
    expect(params.get('from')).toBe('services');
    expect(params.get('search')).toBe('pmg');
    expect(params.get('resource')).toBe('pmg-main');

    const parsed = parseInfrastructureLinkSearch(`?${query}`);
    expect(parsed.source).toBe('pmg');
    expect(parsed.query).toBe('pmg');
    expect(parsed.resource).toBe('pmg-main');
  });

  it('keeps kubernetes redirect metadata while preserving workload deep-link params', () => {
    const target = buildLegacyRedirectTarget(
      LEGACY_REDIRECTS.kubernetes.destination,
      LEGACY_REDIRECTS.kubernetes.source,
    );
    const merged = mergeRedirectQueryParams(
      target,
      '?context=cluster-a&resource=cluster-a:worker-1:0&type=docker&from=bad',
    );

    const [, query = ''] = merged.split('?');
    const params = new URLSearchParams(query);
    expect(params.get('type')).toBe('k8s');
    expect(params.get('migrated')).toBe('1');
    expect(params.get('from')).toBe('kubernetes');
    expect(params.get('context')).toBe('cluster-a');
    expect(params.get('resource')).toBe('cluster-a:worker-1:0');

    const parsed = parseWorkloadsLinkSearch(`?${query}`);
    expect(parsed.type).toBe('k8s');
    expect(parsed.context).toBe('cluster-a');
    expect(parsed.resource).toBe('cluster-a:worker-1:0');
  });
});
