import { describe, expect, it } from 'vitest';
import { getLegacyRedirectByPath, LEGACY_REDIRECTS } from '../legacyRedirects';
import { buildLegacyRedirectTarget } from '../navigation';
import { buildInfrastructurePath, buildWorkloadsPath } from '../resourceLinks';

describe('legacyRedirects contract', () => {
  it('contains expected route mappings', () => {
    expect(LEGACY_REDIRECTS.kubernetes.destination).toBe(buildWorkloadsPath({ type: 'k8s' }));
    expect(LEGACY_REDIRECTS.kubernetes.source).toBe('kubernetes');
    expect(LEGACY_REDIRECTS.kubernetes.toastMessage.toLowerCase()).toContain('deprecated');

    expect(LEGACY_REDIRECTS.services.destination).toBe(buildInfrastructurePath({ source: 'pmg' }));
    expect(LEGACY_REDIRECTS.services.toastMessage.toLowerCase()).toContain('deprecated');
    expect(LEGACY_REDIRECTS.mail.destination).toBe(buildInfrastructurePath({ source: 'pmg' }));
    expect(LEGACY_REDIRECTS.proxmoxOverview.destination).toBe(buildInfrastructurePath());
  });

  it('resolves mappings by path', () => {
    expect(getLegacyRedirectByPath('/kubernetes')?.source).toBe('kubernetes');
    expect(getLegacyRedirectByPath('/services')?.source).toBe('services');
    expect(getLegacyRedirectByPath('/unknown')).toBeNull();
  });

  it('builds redirect targets with migration params from route definitions', () => {
    const k8s = LEGACY_REDIRECTS.kubernetes;
    const services = LEGACY_REDIRECTS.services;
    const k8sTarget = buildLegacyRedirectTarget(k8s.destination, k8s.source);
    const servicesTarget = buildLegacyRedirectTarget(services.destination, services.source);

    expect(k8sTarget).toBe(`${buildWorkloadsPath({ type: 'k8s' })}&migrated=1&from=kubernetes`);
    expect(servicesTarget).toBe(`${buildInfrastructurePath({ source: 'pmg' })}&migrated=1&from=services`);
  });
});
