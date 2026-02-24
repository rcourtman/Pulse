import { describe, expect, it } from 'vitest';
import { LEGACY_ROUTE_MIGRATION_METADATA } from '../legacyRouteMetadata';

describe('legacy route migration metadata', () => {
  it('marks services and kubernetes as deprecated routes', () => {
    expect(LEGACY_ROUTE_MIGRATION_METADATA.services.status.toLowerCase()).toContain('deprecated');
    expect(LEGACY_ROUTE_MIGRATION_METADATA.kubernetes.status.toLowerCase()).toContain('deprecated');
    expect(LEGACY_ROUTE_MIGRATION_METADATA.services.message.toLowerCase()).toContain('deprecated');
    expect(LEGACY_ROUTE_MIGRATION_METADATA.kubernetes.message.toLowerCase()).toContain(
      'deprecated',
    );
  });

  it('keeps legacy compatibility routes marked as compatibility aliases', () => {
    expect(LEGACY_ROUTE_MIGRATION_METADATA['proxmox-overview'].status).toBe(
      'Legacy compatibility alias',
    );
    expect(LEGACY_ROUTE_MIGRATION_METADATA.hosts.status).toBe('Legacy compatibility alias');
    expect(LEGACY_ROUTE_MIGRATION_METADATA.docker.status).toBe('Legacy compatibility alias');
    expect(LEGACY_ROUTE_MIGRATION_METADATA.mail.status).toBe('Legacy compatibility alias');
  });
});
