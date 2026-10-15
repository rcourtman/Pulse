import { describe, expect, it } from 'vitest';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { getPMGAlertResourceIds, getPMGHealth } from '../pmgPresentation';

const alert = (level: Alert['level'], acknowledged = false) => ({ level, acknowledged }) as Alert;

describe('PMG health does not promote partial readings to healthy', () => {
  it.each(['warning', 'degraded', 'partial'])(
    'preserves a %s resource despite a successful instance/version read',
    (status) => {
      expect(getPMGHealth([status, 'healthy', 'online', 'healthy'])).toEqual({
        variant: 'warning',
        label: 'Degraded',
      });
    },
  );
  it.each(['offline', 'error', 'critical', 'down'])(
    'preserves %s from either endpoint',
    (status) => {
      for (const statuses of [
        ['online', status],
        [status, 'healthy'],
      ]) {
        expect(getPMGHealth(statuses, [alert('critical')])).toEqual({
          variant: 'danger',
          label: 'Offline',
        });
      }
    },
  );
  it('keeps failed connection health adverse even when alert detection is disabled', () => {
    expect(getPMGHealth(['online', 'unhealthy'])).toEqual({
      variant: 'danger',
      label: 'Unreachable',
    });
  });
  it('keeps unknown distinct from healthy and ignores absent detail fields', () => {
    expect(getPMGHealth([]).label).toBe('Unknown');
    expect(getPMGHealth(['online', 'unknown']).variant).toBe('muted');
    expect(getPMGHealth(['online', undefined, '']).label).toBe('Healthy');
  });
  it('retains critical and warning attention and clears only acknowledged/resolved alerts', () => {
    expect(getPMGHealth(['online'], [alert('critical')])).toEqual({
      variant: 'danger',
      label: 'Needs attention',
    });
    expect(getPMGHealth(['online'], [alert('warning')]).variant).toBe('warning');
    expect(getPMGHealth(['online'], [alert('critical', true)]).label).toBe('Healthy');
    expect(getPMGHealth(['online'], []).label).toBe('Healthy');
  });
});

describe('PMG alert attribution', () => {
  it('uses explicit typed connection aliases without guessing from names or generated IDs', () => {
    const resource = {
      id: 'unified-1',
      canonicalIdentity: {
        primaryId: 'agent:pmg-gateway',
        supersededIds: ['previous-1'],
        aliases: ['gateway', 'shared.invalid', 'pmg:configured-id', 'agent:host-id'],
      },
      name: 'gateway',
      pmg: { instanceId: 'pmg-gateway', hostname: 'shared.invalid' },
    } as Resource;
    expect(getPMGAlertResourceIds(resource)).toEqual([
      'unified-1',
      'agent:pmg-gateway',
      'previous-1',
      'pmg:configured-id',
      'agent:host-id',
      'pmg-gateway',
    ]);
    expect(getPMGAlertResourceIds(resource)).not.toContain('pmg:gateway');
    expect(getPMGAlertResourceIds(resource)).not.toContain('shared.invalid');
    expect(getPMGAlertResourceIds(resource)).not.toContain('gateway');
  });
});
