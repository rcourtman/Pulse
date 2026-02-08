import { describe, expect, it } from 'vitest';
import { getLegacyRedirectByPath, LEGACY_REDIRECTS } from '../legacyRedirects';
import { buildLegacyRedirectTarget, mergeRedirectQueryParams } from '../navigation';
import { parseInfrastructureLinkSearch, parseWorkloadsLinkSearch } from '../resourceLinks';

describe('legacy route contracts', () => {
  it('covers every legacy redirect definition and preserves migration metadata', () => {
    const infrastructureCases = [
      {
        key: 'proxmoxOverview',
        incoming: '?search=pve-main&resource=node-a',
        expectedSource: '',
        expectedQuery: 'pve-main',
        expectedResource: 'node-a',
      },
      {
        key: 'hosts',
        incoming: '?search=host-2&resource=host-2',
        expectedSource: 'agent',
        expectedQuery: 'host-2',
        expectedResource: 'host-2',
      },
      {
        key: 'docker',
        incoming: '?search=docker-main&resource=docker-main',
        expectedSource: 'docker',
        expectedQuery: 'docker-main',
        expectedResource: 'docker-main',
      },
      {
        key: 'proxmoxMail',
        incoming: '?search=pmg&resource=pmg-main',
        expectedSource: 'pmg',
        expectedQuery: 'pmg',
        expectedResource: 'pmg-main',
      },
      {
        key: 'mail',
        incoming: '?search=pmg&resource=pmg-main',
        expectedSource: 'pmg',
        expectedQuery: 'pmg',
        expectedResource: 'pmg-main',
      },
      {
        key: 'services',
        incoming: '?search=pmg&resource=pmg-main',
        expectedSource: 'pmg',
        expectedQuery: 'pmg',
        expectedResource: 'pmg-main',
      },
    ] as const;

    for (const {
      key,
      incoming,
      expectedSource,
      expectedQuery,
      expectedResource,
    } of infrastructureCases) {
      const redirect = LEGACY_REDIRECTS[key];
      expect(getLegacyRedirectByPath(redirect.path)?.source).toBe(redirect.source);

      const target = buildLegacyRedirectTarget(redirect.destination, redirect.source);
      const merged = mergeRedirectQueryParams(target, incoming);
      const [, query = ''] = merged.split('?');
      const params = new URLSearchParams(query);

      expect(params.get('migrated')).toBe('1');
      expect(params.get('from')).toBe(redirect.source);

      const parsed = parseInfrastructureLinkSearch(`?${query}`);
      expect(parsed.source).toBe(expectedSource);
      expect(parsed.query).toBe(expectedQuery);
      expect(parsed.resource).toBe(expectedResource);
    }

    const kubernetes = LEGACY_REDIRECTS.kubernetes;
    expect(getLegacyRedirectByPath(kubernetes.path)?.source).toBe(kubernetes.source);

    const target = buildLegacyRedirectTarget(kubernetes.destination, kubernetes.source);
    const merged = mergeRedirectQueryParams(
      target,
      '?context=cluster-a&resource=cluster-a:worker-1:0&type=docker',
    );

    const [, query = ''] = merged.split('?');
    const params = new URLSearchParams(query);
    expect(params.get('migrated')).toBe('1');
    expect(params.get('from')).toBe('kubernetes');
    expect(params.get('type')).toBe('k8s');

    const parsed = parseWorkloadsLinkSearch(`?${query}`);
    expect(parsed.type).toBe('k8s');
    expect(parsed.context).toBe('cluster-a');
    expect(parsed.resource).toBe('cluster-a:worker-1:0');
  });

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
