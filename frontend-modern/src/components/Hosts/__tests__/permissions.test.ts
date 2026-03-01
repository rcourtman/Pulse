import { describe, expect, it } from 'vitest';
import { hasSettingsWriteAccess } from '@/components/Hosts/permissions';

describe('hasSettingsWriteAccess', () => {
  it('allows session-based auth with no scoped token', () => {
    expect(hasSettingsWriteAccess(undefined)).toBe(true);
    expect(hasSettingsWriteAccess([])).toBe(true);
  });

  it('allows wildcard-scoped tokens', () => {
    expect(hasSettingsWriteAccess(['*'])).toBe(true);
    expect(hasSettingsWriteAccess(['monitoring:read', '*'])).toBe(true);
  });

  it('allows tokens with explicit settings:write scope', () => {
    expect(hasSettingsWriteAccess(['settings:write'])).toBe(true);
    expect(hasSettingsWriteAccess(['monitoring:read', 'settings:write'])).toBe(true);
  });

  it('denies tokens without settings:write', () => {
    expect(hasSettingsWriteAccess(['monitoring:read'])).toBe(false);
    expect(hasSettingsWriteAccess(['settings:read'])).toBe(false);
    expect(hasSettingsWriteAccess(['monitoring:read', 'host-agent:report'])).toBe(false);
  });
});
