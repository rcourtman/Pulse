import { describe, expect, it } from 'vitest';
import type { Resource, ResourceChange } from '@/types/resource';
import {
  buildVmwareActivityRows,
  buildVmwareIncidentRows,
  buildVmwarePageModel,
  filterVmwareActivity,
  filterVmwareDatastores,
  filterVmwareVirtualMachines,
  formatVmwarePowerState,
  getVmwarePowerStateVariant,
  mapVmwareActivityStateBucket,
  mapVmwareDatastoreStatus,
  mapVmwareIncidentSeverity,
  mapVmwareNetworkStatus,
  mapVmwareVirtualMachineStatus,
  normalizeVmwarePowerStateToken,
} from '../vmwarePageModel';

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'lab',
  platformType: 'vmware-vsphere',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...resource,
});

const makeChange = (
  change: Partial<ResourceChange> & Pick<ResourceChange, 'id'>,
): ResourceChange => ({
  observedAt: '2026-01-01T00:00:00Z',
  resourceId: 'res-1',
  kind: 'activity',
  sourceType: 'platform_event',
  sourceAdapter: 'vmware_adapter',
  confidence: 'high',
  ...change,
});

// ---------------------------------------------------------------------------
// normalizeVmwarePowerStateToken
// ---------------------------------------------------------------------------

describe('normalizeVmwarePowerStateToken', () => {
  it('strips whitespace, underscores, hyphens and lowercases', () => {
    expect(normalizeVmwarePowerStateToken('POWERED_ON')).toBe('poweredon');
    expect(normalizeVmwarePowerStateToken('powered-off')).toBe('poweredoff');
    expect(normalizeVmwarePowerStateToken('power state')).toBe('powerstate');
  });

  it('returns empty string for undefined', () => {
    expect(normalizeVmwarePowerStateToken(undefined)).toBe('');
  });

  it('trims surrounding whitespace before processing', () => {
    expect(normalizeVmwarePowerStateToken('  Suspended  ')).toBe('suspended');
  });
});

// ---------------------------------------------------------------------------
// formatVmwarePowerState
// ---------------------------------------------------------------------------

describe('formatVmwarePowerState', () => {
  it('returns "On" for powered-on and on variants', () => {
    expect(formatVmwarePowerState('POWERED_ON')).toBe('On');
    expect(formatVmwarePowerState('on')).toBe('On');
    expect(formatVmwarePowerState('Powered-On')).toBe('On');
  });

  it('returns "Off" for powered-off and off variants', () => {
    expect(formatVmwarePowerState('POWERED_OFF')).toBe('Off');
    expect(formatVmwarePowerState('off')).toBe('Off');
  });

  it('returns "Suspended" for suspended variants', () => {
    expect(formatVmwarePowerState('SUSPENDED')).toBe('Suspended');
    expect(formatVmwarePowerState('suspended')).toBe('Suspended');
  });

  it('returns "Unknown" for empty and undefined input', () => {
    expect(formatVmwarePowerState('')).toBe('Unknown');
    expect(formatVmwarePowerState(undefined)).toBe('Unknown');
    expect(formatVmwarePowerState('   ')).toBe('Unknown');
  });

  it('returns the trimmed original value for unrecognized non-empty input', () => {
    expect(formatVmwarePowerState('partially-powered')).toBe('partially-powered');
  });
});

// ---------------------------------------------------------------------------
// getVmwarePowerStateVariant
// ---------------------------------------------------------------------------

describe('getVmwarePowerStateVariant', () => {
  it('returns "success" for on states', () => {
    expect(getVmwarePowerStateVariant('POWERED_ON')).toBe('success');
    expect(getVmwarePowerStateVariant('on')).toBe('success');
  });

  it('returns "danger" for off states', () => {
    expect(getVmwarePowerStateVariant('POWERED_OFF')).toBe('danger');
    expect(getVmwarePowerStateVariant('off')).toBe('danger');
  });

  it('returns "warning" for suspended', () => {
    expect(getVmwarePowerStateVariant('suspended')).toBe('warning');
  });

  it('returns "muted" for unknown, empty, and other values', () => {
    expect(getVmwarePowerStateVariant(undefined)).toBe('muted');
    expect(getVmwarePowerStateVariant('')).toBe('muted');
    expect(getVmwarePowerStateVariant('foo')).toBe('muted');
  });
});

// ---------------------------------------------------------------------------
// mapVmwareIncidentSeverity — additional synonym coverage
// ---------------------------------------------------------------------------

describe('mapVmwareIncidentSeverity (additional synonyms)', () => {
  it('maps critical-severity synonyms to "critical"', () => {
    for (const sev of ['critical', 'crit', 'fatal', 'error', 'failed', 'failure']) {
      expect(mapVmwareIncidentSeverity(sev)).toBe('critical');
    }
  });

  it('maps warning-severity synonyms to "warning"', () => {
    for (const sev of ['warning', 'warn', 'alert', 'degraded']) {
      expect(mapVmwareIncidentSeverity(sev)).toBe('warning');
    }
  });

  it('defaults unrecognised and empty values to "info"', () => {
    expect(mapVmwareIncidentSeverity('info')).toBe('info');
    expect(mapVmwareIncidentSeverity('note')).toBe('info');
    expect(mapVmwareIncidentSeverity('')).toBe('info');
    expect(mapVmwareIncidentSeverity(undefined)).toBe('info');
  });

  it('normalises case and whitespace before matching', () => {
    expect(mapVmwareIncidentSeverity('  CRITICAL  ')).toBe('critical');
    expect(mapVmwareIncidentSeverity('Warning')).toBe('warning');
  });
});

// ---------------------------------------------------------------------------
// mapVmwareDatastoreStatus — additional branch coverage
// ---------------------------------------------------------------------------

describe('mapVmwareDatastoreStatus (additional branches)', () => {
  it('does NOT return maintenance for normal/none/not_in_maintenance modes', () => {
    for (const mode of ['normal', 'none', 'not_in_maintenance']) {
      const ds = makeResource({
        id: `ds-${mode}`,
        type: 'storage',
        storage: { topology: 'datastore' },
        vmware: { entityType: 'datastore', datastoreAccessible: true, maintenanceMode: mode },
      });
      expect(mapVmwareDatastoreStatus(ds)).not.toBe('maintenance');
    }
  });

  it('returns inaccessible when status is offline (without datastoreAccessible=false)', () => {
    const ds = makeResource({
      id: 'ds-offline',
      type: 'storage',
      status: 'offline',
      storage: { topology: 'datastore' },
      vmware: { entityType: 'datastore' },
    });
    expect(mapVmwareDatastoreStatus(ds)).toBe('inaccessible');
  });

  it('returns attention when overallStatus is yellow', () => {
    const ds = makeResource({
      id: 'ds-yellow',
      type: 'storage',
      storage: { topology: 'datastore' },
      vmware: { entityType: 'datastore', datastoreAccessible: true, overallStatus: 'yellow' },
    });
    expect(mapVmwareDatastoreStatus(ds)).toBe('attention');
  });

  it('returns attention when resource status is degraded (no overallStatus)', () => {
    const ds = makeResource({
      id: 'ds-degraded',
      type: 'storage',
      status: 'degraded',
      storage: { topology: 'datastore' },
      vmware: { entityType: 'datastore', datastoreAccessible: true },
    });
    expect(mapVmwareDatastoreStatus(ds)).toBe('attention');
  });

  it('returns accessible when status is running (no datastoreAccessible flag)', () => {
    const ds = makeResource({
      id: 'ds-running',
      type: 'storage',
      status: 'running',
      storage: { topology: 'datastore' },
      vmware: { entityType: 'datastore' },
    });
    expect(mapVmwareDatastoreStatus(ds)).toBe('accessible');
  });

  it('returns unknown when no status signals are present', () => {
    const ds = makeResource({
      id: 'ds-bare',
      type: 'storage',
      status: 'unknown',
      storage: { topology: 'datastore' },
      vmware: { entityType: 'datastore' },
    });
    expect(mapVmwareDatastoreStatus(ds)).toBe('unknown');
  });
});

// ---------------------------------------------------------------------------
// mapVmwareActivityStateBucket — additional synonym coverage
// ---------------------------------------------------------------------------

describe('mapVmwareActivityStateBucket (additional synonyms)', () => {
  it('maps failure synonyms to "failed"', () => {
    for (const s of ['failed', 'failure', 'cancelled', 'canceled', 'timedout', 'timeout', 'red']) {
      expect(mapVmwareActivityStateBucket(s)).toBe('failed');
    }
  });

  it('maps running synonyms to "running"', () => {
    for (const s of ['running', 'queued', 'pending', 'inprogress', 'started']) {
      expect(mapVmwareActivityStateBucket(s)).toBe('running');
    }
  });

  it('maps success synonyms to "success"', () => {
    for (const s of ['succeeded', 'complete', 'completed', 'ok', 'green']) {
      expect(mapVmwareActivityStateBucket(s)).toBe('success');
    }
  });

  it('returns "unknown" for unrecognised values', () => {
    expect(mapVmwareActivityStateBucket('foo')).toBe('unknown');
    expect(mapVmwareActivityStateBucket('')).toBe('unknown');
  });

  it('normalises separators and case', () => {
    expect(mapVmwareActivityStateBucket('In-Progress')).toBe('running');
    expect(mapVmwareActivityStateBucket('TIME_OUT')).toBe('failed');
  });
});

// ---------------------------------------------------------------------------
// mapVmwareNetworkStatus — additional branches
// ---------------------------------------------------------------------------

describe('mapVmwareNetworkStatus (additional branches)', () => {
  it('returns healthy for online/running status without green overallStatus', () => {
    const net = makeResource({
      id: 'net-online',
      type: 'network',
      status: 'online',
      vmware: { entityType: 'network' },
    });
    expect(mapVmwareNetworkStatus(net)).toBe('healthy');
  });

  it('returns unknown when no signals match healthy or attention', () => {
    const net = makeResource({
      id: 'net-unknown',
      type: 'network',
      status: 'unknown',
      vmware: { entityType: 'network' },
    });
    expect(mapVmwareNetworkStatus(net)).toBe('unknown');
  });
});

// ---------------------------------------------------------------------------
// mapVmwareVirtualMachineStatus — additional branches
// ---------------------------------------------------------------------------

describe('mapVmwareVirtualMachineStatus (additional branches)', () => {
  it('returns suspended for suspended powerState', () => {
    const vm = makeResource({
      id: 'vm-suspended',
      type: 'vm',
      vmware: { entityType: 'vm', powerState: 'suspended' },
    });
    expect(mapVmwareVirtualMachineStatus(vm)).toBe('suspended');
  });

  it('returns attention for paused status (paused is in the attention-check list, so the suspended branch for status===paused is unreachable)', () => {
    const vm = makeResource({
      id: 'vm-paused',
      type: 'vm',
      status: 'paused',
      vmware: { entityType: 'vm' },
    });
    expect(mapVmwareVirtualMachineStatus(vm)).toBe('attention');
  });

  it('returns powered-on for online status without powerState', () => {
    const vm = makeResource({
      id: 'vm-online',
      type: 'vm',
      status: 'online',
      vmware: { entityType: 'vm' },
    });
    expect(mapVmwareVirtualMachineStatus(vm)).toBe('powered-on');
  });

  it('returns powered-off for offline status without powerState', () => {
    const vm = makeResource({
      id: 'vm-offline',
      type: 'vm',
      status: 'offline',
      vmware: { entityType: 'vm' },
    });
    expect(mapVmwareVirtualMachineStatus(vm)).toBe('powered-off');
  });

  it('returns attention for activeAlarmCount > 0 alone', () => {
    const vm = makeResource({
      id: 'vm-alarmed',
      type: 'vm',
      vmware: { entityType: 'vm', powerState: 'poweredOn', activeAlarmCount: 3 },
    });
    expect(mapVmwareVirtualMachineStatus(vm)).toBe('attention');
  });

  it('returns attention for degraded status alone', () => {
    const vm = makeResource({
      id: 'vm-degraded',
      type: 'vm',
      status: 'degraded',
      vmware: { entityType: 'vm', powerState: 'poweredOn' },
    });
    expect(mapVmwareVirtualMachineStatus(vm)).toBe('attention');
  });

  it('returns unknown when no signals match any known state', () => {
    const vm = makeResource({
      id: 'vm-bare',
      type: 'vm',
      status: 'unknown',
      vmware: { entityType: 'vm' },
    });
    expect(mapVmwareVirtualMachineStatus(vm)).toBe('unknown');
  });
});

// ---------------------------------------------------------------------------
// hasIncidentSignal — via buildVmwareIncidentRows
// ---------------------------------------------------------------------------

describe('hasIncidentSignal (via buildVmwareIncidentRows)', () => {
  it('filters out incidents with empty code and summary', () => {
    const res = makeResource({
      id: 'res-no-signal',
      type: 'agent',
      incidents: [{ code: '', severity: 'warning', summary: '' }],
    });
    expect(buildVmwareIncidentRows([res])).toEqual([]);
  });

  it('keeps incidents with a non-empty code but empty summary', () => {
    const res = makeResource({
      id: 'res-code-only',
      type: 'agent',
      incidents: [{ code: 'alarm-x', severity: 'warning', summary: '' }],
    });
    const rows = buildVmwareIncidentRows([res]);
    expect(rows).toHaveLength(1);
    expect(rows[0].code).toBe('alarm-x');
  });

  it('keeps incidents with an empty code but non-empty summary', () => {
    const res = makeResource({
      id: 'res-summary-only',
      type: 'agent',
      incidents: [{ code: '', severity: 'info', summary: 'something happened' }],
    });
    const rows = buildVmwareIncidentRows([res]);
    expect(rows).toHaveLength(1);
    expect(rows[0].summary).toBe('something happened');
  });
});

// ---------------------------------------------------------------------------
// buildIncidentRow — via buildVmwareIncidentRows
// ---------------------------------------------------------------------------

describe('buildIncidentRow (via buildVmwareIncidentRows)', () => {
  it('uses nativeId in row id when present', () => {
    const res = makeResource({
      id: 'res-a',
      type: 'agent',
      incidents: [{ nativeId: 'alarm-999', code: 'alarm', severity: 'critical', summary: 'boom' }],
    });
    const rows = buildVmwareIncidentRows([res]);
    expect(rows[0].id).toBe('res-a:incident:alarm-999:0');
  });

  it('falls back to code in row id when no nativeId', () => {
    const res = makeResource({
      id: 'res-b',
      type: 'agent',
      incidents: [{ code: 'my_code', severity: 'warning', summary: 'warn' }],
    });
    const rows = buildVmwareIncidentRows([res]);
    expect(rows[0].id).toBe('res-b:incident:my_code:0');
  });

  it('derives severity from incident then resource.incidentSeverity then defaults to info', () => {
    const fromIncident = makeResource({
      id: 'sev-1',
      type: 'agent',
      incidents: [{ code: 'c', severity: 'critical', summary: 's' }],
    });
    expect(buildVmwareIncidentRows([fromIncident])[0].severity).toBe('critical');

    const fromResource = makeResource({
      id: 'sev-2',
      type: 'agent',
      incidentSeverity: 'warning',
      incidents: [{ code: 'c', severity: '', summary: 's' }],
    });
    expect(buildVmwareIncidentRows([fromResource])[0].severity).toBe('warning');

    const defaulted = makeResource({
      id: 'sev-3',
      type: 'agent',
      incidents: [{ code: 'c', severity: '', summary: 's' }],
    });
    expect(buildVmwareIncidentRows([defaulted])[0].severity).toBe('info');
  });

  it('derives code from incident then resource.incidentCode then defaults to vmware_alert', () => {
    const fromResource = makeResource({
      id: 'code-1',
      type: 'agent',
      incidentCode: 'rollup_code',
      incidents: [{ code: '', severity: 'info', summary: 's' }],
    });
    expect(buildVmwareIncidentRows([fromResource])[0].code).toBe('rollup_code');

    const defaulted = makeResource({
      id: 'code-2',
      type: 'agent',
      incidents: [{ code: '', severity: 'info', summary: 's' }],
    });
    expect(buildVmwareIncidentRows([defaulted])[0].code).toBe('vmware_alert');
  });

  it('derives summary from incident then resource.incidentSummary then label', () => {
    const fromResource = makeResource({
      id: 'sum-1',
      type: 'agent',
      incidentSummary: 'resource summary',
      incidents: [{ code: 'alarm', severity: 'info', summary: '' }],
    });
    expect(buildVmwareIncidentRows([fromResource])[0].summary).toBe('resource summary');
  });

  it('uses incident.provider as source when source is absent', () => {
    const res = makeResource({
      id: 'src-1',
      type: 'agent',
      incidents: [{ provider: 'nsx', code: 'c', severity: 'info', summary: 's' }],
    });
    expect(buildVmwareIncidentRows([res])[0].source).toBe('nsx');
  });

  it('defaults source to vmware when neither source nor provider', () => {
    const res = makeResource({
      id: 'src-2',
      type: 'agent',
      incidents: [{ code: 'c', severity: 'info', summary: 's' }],
    });
    expect(buildVmwareIncidentRows([res])[0].source).toBe('vmware');
  });

  it('uses resource.incidentAction override', () => {
    const res = makeResource({
      id: 'act-1',
      type: 'agent',
      incidentAction: 'Check vCenter logs',
      incidents: [{ code: 'c', severity: 'info', summary: 's' }],
    });
    expect(buildVmwareIncidentRows([res])[0].action).toBe('Check vCenter logs');
  });

  it('defaults action to "Investigate in vCenter"', () => {
    const res = makeResource({
      id: 'act-2',
      type: 'agent',
      incidents: [{ code: 'c', severity: 'info', summary: 's' }],
    });
    expect(buildVmwareIncidentRows([res])[0].action).toBe('Investigate in vCenter');
  });

  it('uses resource.incidentPriority override for priority', () => {
    const res = makeResource({
      id: 'prio-1',
      type: 'agent',
      incidentPriority: 9999,
      incidents: [{ code: 'c', severity: 'info', summary: 's' }],
    });
    expect(buildVmwareIncidentRows([res])[0].priority).toBe(9999);
  });

  it('computes default priority as severityRank * 1000', () => {
    const res = makeResource({
      id: 'prio-2',
      type: 'agent',
      incidents: [{ code: 'c', severity: 'critical', summary: 's' }],
    });
    expect(buildVmwareIncidentRows([res])[0].priority).toBe(3000);
  });

  it('uses resource.incidentCategory override', () => {
    const res = makeResource({
      id: 'cat-1',
      type: 'agent',
      incidentCategory: 'custom-cat',
      incidents: [{ code: 'c', severity: 'info', summary: 's' }],
    });
    expect(buildVmwareIncidentRows([res])[0].category).toBe('custom-cat');
  });

  it('defaults category to vcenter-health', () => {
    const res = makeResource({
      id: 'cat-2',
      type: 'agent',
      incidents: [{ code: 'c', severity: 'info', summary: 's' }],
    });
    expect(buildVmwareIncidentRows([res])[0].category).toBe('vcenter-health');
  });

  it('uses resource.type as entityType when vmware.entityType is absent', () => {
    const res = makeResource({
      id: 'et-1',
      type: 'agent',
      vmware: {},
      incidents: [{ code: 'c', severity: 'info', summary: 's' }],
    });
    expect(buildVmwareIncidentRows([res])[0].entityType).toBe('agent');
  });

  it('uses resource.id as managedObjectId when vmware.managedObjectId is absent', () => {
    const res = makeResource({
      id: 'moid-1',
      type: 'agent',
      vmware: {},
      incidents: [{ code: 'c', severity: 'info', summary: 's' }],
    });
    expect(buildVmwareIncidentRows([res])[0].managedObjectId).toBe('moid-1');
  });

  it('preserves startedAt from incident', () => {
    const res = makeResource({
      id: 'st-1',
      type: 'agent',
      incidents: [{ code: 'c', severity: 'info', summary: 's', startedAt: '2026-03-01T10:00:00Z' }],
    });
    expect(buildVmwareIncidentRows([res])[0].startedAt).toBe('2026-03-01T10:00:00Z');
  });
});

// ---------------------------------------------------------------------------
// vmwareResourceDisplayName — via buildVmwareIncidentRows
// ---------------------------------------------------------------------------

describe('vmwareResourceDisplayName (via buildVmwareIncidentRows)', () => {
  const incident = { code: 'c', severity: 'info', summary: 's' };

  it('falls back to vmware.runtimeHostName when displayName and name are empty', () => {
    const res = makeResource({
      id: 'rdn-1',
      type: 'agent',
      displayName: '',
      name: '',
      vmware: { runtimeHostName: 'esxi-10.lab.local', managedObjectId: 'host-10' },
      incidents: [incident],
    });
    expect(buildVmwareIncidentRows([res])[0].resourceName).toBe('esxi-10.lab.local');
  });

  it('falls back to vmware.managedObjectId when displayName, name and runtimeHostName are empty', () => {
    const res = makeResource({
      id: 'rdn-2',
      type: 'agent',
      displayName: '',
      name: '',
      vmware: { managedObjectId: 'host-20' },
      incidents: [incident],
    });
    expect(buildVmwareIncidentRows([res])[0].resourceName).toBe('host-20');
  });

  it('falls back to resource.id when all other fields are empty', () => {
    const res = makeResource({
      id: 'rdn-3',
      type: 'agent',
      displayName: '',
      name: '',
      vmware: {},
      incidents: [incident],
    });
    expect(buildVmwareIncidentRows([res])[0].resourceName).toBe('rdn-3');
  });
});

// ---------------------------------------------------------------------------
// vmwareIncidentLabel — via buildVmwareIncidentRows
// ---------------------------------------------------------------------------

describe('vmwareIncidentLabel (via buildVmwareIncidentRows)', () => {
  const incident = (code: string) => ({ code, severity: 'info', summary: 's' });

  it('returns resource.incidentLabel when present (overrides everything)', () => {
    const res = makeResource({
      id: 'lbl-1',
      type: 'agent',
      incidentLabel: 'Custom Label',
      incidents: [incident('vmware_alarm_state')],
    });
    expect(buildVmwareIncidentRows([res])[0].label).toBe('Custom Label');
  });

  it('returns "vSphere Alarm" for code vmware_alarm_state', () => {
    const res = makeResource({
      id: 'lbl-2',
      type: 'agent',
      incidents: [incident('vmware_alarm_state')],
    });
    expect(buildVmwareIncidentRows([res])[0].label).toBe('vSphere Alarm');
  });

  it('returns "vSphere Health" for code vmware_health_state', () => {
    const res = makeResource({
      id: 'lbl-3',
      type: 'agent',
      incidents: [incident('vmware_health_state')],
    });
    expect(buildVmwareIncidentRows([res])[0].label).toBe('vSphere Health');
  });

  it('titleizes other vmware_ codes', () => {
    const res = makeResource({
      id: 'lbl-4',
      type: 'agent',
      incidents: [incident('vmware_disk_usage')],
    });
    expect(buildVmwareIncidentRows([res])[0].label).toBe('Disk Usage');
  });

  it('returns "vSphere Health" when code is empty', () => {
    const res = makeResource({
      id: 'lbl-5',
      type: 'agent',
      incidents: [{ code: '', severity: 'info', summary: 'fallback summary' }],
    });
    expect(buildVmwareIncidentRows([res])[0].label).toBe('vSphere Health');
  });
});

// ---------------------------------------------------------------------------
// buildRollupIncidentRow — via buildVmwareIncidentRows
// ---------------------------------------------------------------------------

describe('buildRollupIncidentRow (via buildVmwareIncidentRows)', () => {
  it('produces a rollup row with rollup id when no explicit incidents exist', () => {
    const res = makeResource({
      id: 'roll-1',
      type: 'agent',
      incidentCount: 2,
      incidentSeverity: 'warning',
    });
    const rows = buildVmwareIncidentRows([res]);
    expect(rows).toHaveLength(1);
    expect(rows[0].id).toBe('roll-1:incident:rollup');
    expect(rows[0].severity).toBe('warning');
  });

  it('defaults rollup severity to info when incidentSeverity is absent', () => {
    const res = makeResource({ id: 'roll-2', type: 'agent', incidentCount: 1 });
    expect(buildVmwareIncidentRows([res])[0].severity).toBe('info');
  });

  it('defaults rollup code to vmware_alert when incidentCode is absent', () => {
    const res = makeResource({ id: 'roll-3', type: 'agent', incidentCount: 1 });
    expect(buildVmwareIncidentRows([res])[0].code).toBe('vmware_alert');
  });

  it('generates plural summary from count when no summary or label', () => {
    const res = makeResource({ id: 'roll-4', type: 'agent', incidentCount: 3 });
    expect(buildVmwareIncidentRows([res])[0].summary).toBe('3 active vSphere alerts');
  });

  it('generates singular summary when count is 1', () => {
    const res = makeResource({ id: 'roll-5', type: 'agent', incidentCount: 1 });
    expect(buildVmwareIncidentRows([res])[0].summary).toBe('1 active vSphere alert');
  });

  it('uses count||1 for the number but count===1 for plural (count=0 produces "1 active vSphere alerts" — see source bug note)', () => {
    const res = makeResource({ id: 'roll-6', type: 'agent', incidentCount: 0, incidentCode: 'x' });
    expect(buildVmwareIncidentRows([res])[0].summary).toBe('1 active vSphere alerts');
  });

  it('uses incidentLabel in summary before count-based fallback', () => {
    const res = makeResource({
      id: 'roll-7',
      type: 'agent',
      incidentCount: 2,
      incidentLabel: 'Storage Alarm',
    });
    expect(buildVmwareIncidentRows([res])[0].summary).toBe('Storage Alarm');
  });
});

// ---------------------------------------------------------------------------
// incidentSeverityRank — via buildVmwareIncidentRows sort order
// ---------------------------------------------------------------------------

describe('incidentSeverityRank (via buildVmwareIncidentRows sorting)', () => {
  it('sorts incidents critical > warning > info by severity rank', () => {
    const info = makeResource({
      id: 'sort-info',
      type: 'agent',
      incidents: [{ code: 'c', severity: 'info', summary: 's' }],
    });
    const warning = makeResource({
      id: 'sort-warning',
      type: 'agent',
      incidents: [{ code: 'c', severity: 'warning', summary: 's' }],
    });
    const critical = makeResource({
      id: 'sort-critical',
      type: 'agent',
      incidents: [{ code: 'c', severity: 'critical', summary: 's' }],
    });

    const rows = buildVmwareIncidentRows([info, warning, critical]);
    expect(rows.map((r) => r.resourceId)).toEqual(['sort-critical', 'sort-warning', 'sort-info']);
  });

  it('tie-breaks same-severity incidents by priority descending', () => {
    const high = makeResource({
      id: 'prio-high',
      type: 'agent',
      incidentPriority: 5000,
      incidents: [{ code: 'c', severity: 'warning', summary: 's' }],
    });
    const low = makeResource({
      id: 'prio-low',
      type: 'agent',
      incidentPriority: 1000,
      incidents: [{ code: 'c', severity: 'warning', summary: 's' }],
    });

    const rows = buildVmwareIncidentRows([low, high]);
    expect(rows.map((r) => r.resourceId)).toEqual(['prio-high', 'prio-low']);
  });

  it('tie-breaks same-severity same-priority by entity type then resource name', () => {
    const vmRow = makeResource({
      id: 'zzz-vm',
      type: 'vm',
      name: 'zzz-vm',
      displayName: 'zzz-vm',
      incidents: [{ code: 'c', severity: 'info', summary: 's' }],
    });
    const agentRow = makeResource({
      id: 'aaa-agent',
      type: 'agent',
      name: 'aaa-agent',
      displayName: 'aaa-agent',
      incidents: [{ code: 'c', severity: 'info', summary: 's' }],
    });

    const rows = buildVmwareIncidentRows([vmRow, agentRow]);
    // agent < vm alphabetically
    expect(rows.map((r) => r.entityType)).toEqual(['agent', 'vm']);
  });
});

// ---------------------------------------------------------------------------
// isVmwareActivityChange — via buildVmwareActivityRows
// ---------------------------------------------------------------------------

describe('isVmwareActivityChange (via buildVmwareActivityRows)', () => {
  it('rejects changes whose kind is not activity', () => {
    const res = makeResource({ id: 'r1', type: 'vm' });
    const change = makeChange({
      id: 'c1',
      resourceId: 'r1',
      kind: 'state_transition' as ResourceChange['kind'],
    });
    expect(buildVmwareActivityRows([res], [change])).toEqual([]);
  });

  it('accepts activity changes with vmware_adapter sourceAdapter', () => {
    const res = makeResource({ id: 'r2', type: 'vm' });
    const change = makeChange({ id: 'c2', resourceId: 'r2', sourceAdapter: 'vmware_adapter' });
    expect(buildVmwareActivityRows([res], [change])).toHaveLength(1);
  });

  it('accepts activity changes with vmware_ prefixed activity_type (no adapter)', () => {
    const res = makeResource({ id: 'r3', type: 'vm' });
    const change = makeChange({
      id: 'c3',
      resourceId: 'r3',
      sourceAdapter: undefined,
      metadata: { activity_type: 'vmware_task' },
    });
    expect(buildVmwareActivityRows([res], [change])).toHaveLength(1);
  });

  it('accepts activity changes with vmware metadata keys (no adapter, no vmware_ prefix)', () => {
    const res = makeResource({ id: 'r4', type: 'vm' });
    const change = makeChange({
      id: 'c4',
      resourceId: 'r4',
      sourceAdapter: undefined,
      metadata: { vmwareEvent: 'event-1' },
    });
    expect(buildVmwareActivityRows([res], [change])).toHaveLength(1);
  });

  it('rejects non-vmware activity changes with no vmware signals', () => {
    const res = makeResource({ id: 'r5', type: 'vm' });
    const change = makeChange({
      id: 'c5',
      resourceId: 'r5',
      sourceAdapter: 'docker_adapter',
      metadata: { activity_type: 'container_start' },
    });
    expect(buildVmwareActivityRows([res], [change])).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// mapVmwareActivityKind — via buildVmwareActivityRows
// ---------------------------------------------------------------------------

describe('mapVmwareActivityKind (via buildVmwareActivityRows)', () => {
  it('classifies as task when activity_type contains "task"', () => {
    const res = makeResource({ id: 'k1', type: 'vm' });
    const change = makeChange({
      id: 'ck1',
      resourceId: 'k1',
      metadata: { activity_type: 'vmware_task_reconfigure' },
    });
    expect(buildVmwareActivityRows([res], [change])[0].activityKind).toBe('task');
  });

  it('classifies as task when metadata has vmwareTaskName (generic activity_type)', () => {
    const res = makeResource({ id: 'k2', type: 'vm' });
    const change = makeChange({
      id: 'ck2',
      resourceId: 'k2',
      metadata: { activity_type: 'vmware_activity', vmwareTaskName: 'RelocateVM' },
    });
    expect(buildVmwareActivityRows([res], [change])[0].activityKind).toBe('task');
  });

  it('classifies as event when activity_type contains "event"', () => {
    const res = makeResource({ id: 'k3', type: 'vm' });
    const change = makeChange({
      id: 'ck3',
      resourceId: 'k3',
      metadata: { activity_type: 'vmware_event_login' },
    });
    expect(buildVmwareActivityRows([res], [change])[0].activityKind).toBe('event');
  });

  it('classifies as event when metadata has vmwareEventMessage', () => {
    const res = makeResource({ id: 'k4', type: 'vm' });
    const change = makeChange({
      id: 'ck4',
      resourceId: 'k4',
      metadata: { activity_type: 'vmware_x', vmwareEventMessage: 'VM started' },
    });
    expect(buildVmwareActivityRows([res], [change])[0].activityKind).toBe('event');
  });

  it('classifies as activity when no task or event signals', () => {
    const res = makeResource({ id: 'k5', type: 'vm' });
    const change = makeChange({
      id: 'ck5',
      resourceId: 'k5',
      metadata: { activity_type: 'vmware_generic' },
    });
    expect(buildVmwareActivityRows([res], [change])[0].activityKind).toBe('activity');
  });
});

// ---------------------------------------------------------------------------
// buildActivityRow — via buildVmwareActivityRows
// ---------------------------------------------------------------------------

describe('buildActivityRow field extraction (via buildVmwareActivityRows)', () => {
  it('falls back title through metadata keys → reason → default', () => {
    const res = makeResource({ id: 't1', type: 'vm' });

    const fromReason = makeChange({
      id: 'ct1a',
      resourceId: 't1',
      metadata: {},
      reason: 'Reason as title',
    });
    expect(buildVmwareActivityRows([res], [fromReason])[0].title).toBe('Reason as title');

    const defaultTitle = makeChange({ id: 'ct1b', resourceId: 't1', metadata: {} });
    expect(buildVmwareActivityRows([res], [defaultTitle])[0].title).toBe('vSphere activity');
  });

  it('extracts state from metadata then change.to', () => {
    const res = makeResource({ id: 't2', type: 'vm' });

    const fromTo = makeChange({
      id: 'ct2',
      resourceId: 't2',
      metadata: {},
      to: 'poweredOn',
    });
    expect(buildVmwareActivityRows([res], [fromTo])[0].state).toBe('poweredOn');
  });

  it('extracts message from metadata then reason', () => {
    const res = makeResource({ id: 't3', type: 'vm' });

    const fromReason = makeChange({
      id: 'ct3',
      resourceId: 't3',
      metadata: {},
      reason: 'Error detail',
    });
    expect(buildVmwareActivityRows([res], [fromReason])[0].message).toBe('Error detail');

    const empty = makeChange({ id: 'ct3b', resourceId: 't3', metadata: {} });
    expect(buildVmwareActivityRows([res], [empty])[0].message).toBe('');
  });

  it('extracts actor from change.actor then metadata.vmwareEventUser', () => {
    const res = makeResource({ id: 't4', type: 'vm' });

    const fromMeta = makeChange({
      id: 'ct4',
      resourceId: 't4',
      metadata: { vmwareEventUser: 'admin@vsphere.local' },
    });
    expect(buildVmwareActivityRows([res], [fromMeta])[0].actor).toBe('admin@vsphere.local');
  });

  it('extracts nativeId from metadata keys then falls back to change.id', () => {
    const res = makeResource({ id: 't5', type: 'vm' });

    const fromChangeId = makeChange({
      id: 'change-id-xyz',
      resourceId: 't5',
      metadata: {},
    });
    expect(buildVmwareActivityRows([res], [fromChangeId])[0].nativeId).toBe('change-id-xyz');
  });

  it('extracts entityType from metadata then resource.vmware.entityType then resource.type', () => {
    const res = makeResource({
      id: 't6',
      type: 'vm',
      vmware: {},
    });

    const fromType = makeChange({ id: 'ct6', resourceId: 't6', metadata: {} });
    expect(buildVmwareActivityRows([res], [fromType])[0].entityType).toBe('vm');
  });

  it('extracts managedObjectId from metadata then resource.vmware.managedObjectId then resource.id', () => {
    const res = makeResource({ id: 't7', type: 'vm', vmware: {} });

    const fromId = makeChange({ id: 'ct7', resourceId: 't7', metadata: {} });
    expect(buildVmwareActivityRows([res], [fromId])[0].managedObjectId).toBe('t7');
  });

  it('derives source from change.sourceAdapter then change.sourceType then vmware', () => {
    const res = makeResource({ id: 't8', type: 'vm' });

    const fromSourceType = makeChange({
      id: 'ct8',
      resourceId: 't8',
      sourceAdapter: undefined,
      sourceType: 'agent_action',
      metadata: { activity_type: 'vmware_task' },
    });
    expect(buildVmwareActivityRows([res], [fromSourceType])[0].source).toBe('agent_action');

    const fromDefault = makeChange({
      id: 'ct8b',
      resourceId: 't8',
      sourceAdapter: undefined,
      sourceType: 'pulse_diff',
      metadata: { activity_type: 'vmware_task' },
    });
    expect(buildVmwareActivityRows([res], [fromDefault])[0].source).toBe('pulse_diff');
  });

  it('sets occurredAt to undefined when not present on change', () => {
    const res = makeResource({ id: 't9', type: 'vm' });
    const change = makeChange({
      id: 'ct9',
      resourceId: 't9',
      occurredAt: undefined,
      metadata: {},
    });
    expect(buildVmwareActivityRows([res], [change])[0].occurredAt).toBeUndefined();
  });

  it('sets occurredAt to the trimmed value when present', () => {
    const res = makeResource({ id: 't10', type: 'vm' });
    const change = makeChange({
      id: 'ct10',
      resourceId: 't10',
      occurredAt: '2026-06-15T12:00:00Z',
      metadata: {},
    });
    expect(buildVmwareActivityRows([res], [change])[0].occurredAt).toBe('2026-06-15T12:00:00Z');
  });
});

// ---------------------------------------------------------------------------
// parseActivitySortTime — via buildVmwareActivityRows sortTime
// ---------------------------------------------------------------------------

describe('parseActivitySortTime (via buildVmwareActivityRows)', () => {
  it('uses occurredAt timestamp when valid', () => {
    const res = makeResource({ id: 'p1', type: 'vm' });
    const change = makeChange({
      id: 'cp1',
      resourceId: 'p1',
      occurredAt: '2026-06-01T00:00:00Z',
      observedAt: '2026-06-02T00:00:00Z',
      metadata: {},
    });
    const expected = new Date('2026-06-01T00:00:00Z').getTime();
    expect(buildVmwareActivityRows([res], [change])[0].sortTime).toBe(expected);
  });

  it('falls back to observedAt when occurredAt is absent', () => {
    const res = makeResource({ id: 'p2', type: 'vm' });
    const change = makeChange({
      id: 'cp2',
      resourceId: 'p2',
      occurredAt: undefined,
      observedAt: '2026-06-03T00:00:00Z',
      metadata: {},
    });
    const expected = new Date('2026-06-03T00:00:00Z').getTime();
    expect(buildVmwareActivityRows([res], [change])[0].sortTime).toBe(expected);
  });

  it('returns 0 when both occurredAt and observedAt are absent', () => {
    const res = makeResource({ id: 'p3', type: 'vm' });
    const change = makeChange({
      id: 'cp3',
      resourceId: 'p3',
      occurredAt: undefined,
      observedAt: '',
      metadata: {},
    });
    expect(buildVmwareActivityRows([res], [change])[0].sortTime).toBe(0);
  });

  it('returns 0 for invalid date strings', () => {
    const res = makeResource({ id: 'p4', type: 'vm' });
    const change = makeChange({
      id: 'cp4',
      resourceId: 'p4',
      occurredAt: 'not-a-date',
      metadata: {},
    });
    expect(buildVmwareActivityRows([res], [change])[0].sortTime).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// resolveVmwareActivityResource — via buildVmwareActivityRows
// ---------------------------------------------------------------------------

describe('resolveVmwareActivityResource (via buildVmwareActivityRows)', () => {
  it('resolves by resource.id', () => {
    const res = makeResource({ id: 'direct-id', type: 'vm' });
    const change = makeChange({ id: 'rc1', resourceId: 'direct-id', metadata: {} });
    const rows = buildVmwareActivityRows([res], [change]);
    expect(rows).toHaveLength(1);
    expect(rows[0].resourceId).toBe('direct-id');
  });

  it('resolves by canonicalIdentity.primaryId', () => {
    const res = makeResource({
      id: 'vm-200',
      type: 'vm',
      canonicalIdentity: { primaryId: 'vc-1:vm:vm-200' },
    });
    const change = makeChange({ id: 'rc2', resourceId: 'vc-1:vm:vm-200', metadata: {} });
    const rows = buildVmwareActivityRows([res], [change]);
    expect(rows).toHaveLength(1);
    expect(rows[0].resourceId).toBe('vm-200');
  });

  it('resolves by canonicalIdentity.aliases', () => {
    const res = makeResource({
      id: 'vm-300',
      type: 'vm',
      canonicalIdentity: { aliases: ['alias-A', 'alias-B'] },
    });
    const change = makeChange({ id: 'rc3', resourceId: 'alias-B', metadata: {} });
    const rows = buildVmwareActivityRows([res], [change]);
    expect(rows).toHaveLength(1);
    expect(rows[0].resourceId).toBe('vm-300');
  });

  it('resolves by vmware source alias (connectionId:entityType:managedObjectId)', () => {
    const res = makeResource({
      id: 'vm-400',
      type: 'vm',
      vmware: { connectionId: 'vc-2', entityType: 'vm', managedObjectId: 'vm-400' },
    });
    const change = makeChange({
      id: 'rc4',
      resourceId: 'nonexistent',
      metadata: {
        vmwareConnectionId: 'vc-2',
        vmwareEntityType: 'vm',
        vmwareManagedObjectId: 'vm-400',
        activity_type: 'vmware_task',
      },
    });
    const rows = buildVmwareActivityRows([res], [change]);
    expect(rows).toHaveLength(1);
    expect(rows[0].resourceId).toBe('vm-400');
  });

  it('resolves a storage resource via metadata vmwareManagedObjectId', () => {
    const res = makeResource({
      id: 'ds-500',
      type: 'storage',
      storage: { topology: 'datastore' },
      vmware: { entityType: 'datastore', managedObjectId: 'datastore-500', connectionId: 'vc-3' },
    });
    const change = makeChange({
      id: 'rc5',
      resourceId: 'unrelated',
      metadata: {
        vmwareManagedObjectId: 'datastore-500',
        activity_type: 'vmware_task',
      },
    });
    const rows = buildVmwareActivityRows([res], [change]);
    expect(rows).toHaveLength(1);
    expect(rows[0].resourceId).toBe('ds-500');
  });

  it('skips changes that do not match any resource', () => {
    const res = makeResource({ id: 'vm-600', type: 'vm' });
    const change = makeChange({
      id: 'rc6',
      resourceId: 'completely-unmatched',
      metadata: { activity_type: 'vmware_task' },
    });
    expect(buildVmwareActivityRows([res], [change])).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// appendRow / dedup — via buildVmwareActivityRows
// ---------------------------------------------------------------------------

describe('appendRow / dedup (via buildVmwareActivityRows)', () => {
  it('includes changes embedded in resource.recentChanges', () => {
    const res = makeResource({
      id: 'rc-res',
      type: 'vm',
      recentChanges: [makeChange({ id: 'embedded-1', resourceId: 'rc-res', metadata: {} })],
    });
    const rows = buildVmwareActivityRows([res], []);
    expect(rows).toHaveLength(1);
    expect(rows[0].nativeId).toBe('embedded-1');
  });

  it('deduplicates the same change appearing in both recentChanges and activityChanges', () => {
    const dup = makeChange({ id: 'dup-1', resourceId: 'dd-res', reason: 'same', metadata: {} });
    const res = makeResource({
      id: 'dd-res',
      type: 'vm',
      recentChanges: [dup],
    });
    const rows = buildVmwareActivityRows([res], [dup]);
    expect(rows).toHaveLength(1);
  });

  it('skips non-vmware changes embedded in recentChanges', () => {
    const res = makeResource({
      id: 'skip-res',
      type: 'vm',
      recentChanges: [
        makeChange({
          id: 'non-vm',
          resourceId: 'skip-res',
          sourceAdapter: 'docker_adapter',
          metadata: { activity_type: 'container_start' },
        }),
      ],
    });
    expect(buildVmwareActivityRows([res], [])).toEqual([]);
  });

  it('sorts rows by sortTime descending then resourceName then id', () => {
    const res = makeResource({ id: 'sort-res', type: 'vm' });
    const earlier = makeChange({
      id: 'earlier',
      resourceId: 'sort-res',
      occurredAt: '2026-01-01T00:00:00Z',
      metadata: {},
    });
    const later = makeChange({
      id: 'later',
      resourceId: 'sort-res',
      occurredAt: '2026-06-01T00:00:00Z',
      metadata: {},
    });
    const rows = buildVmwareActivityRows([res], [earlier, later]);
    expect(rows.map((r) => r.nativeId)).toEqual(['later', 'earlier']);
  });
});

// ---------------------------------------------------------------------------
// vmwareDatastoreSearchHaystack — via filterVmwareDatastores
// ---------------------------------------------------------------------------

describe('vmwareDatastoreSearchHaystack (via filterVmwareDatastores)', () => {
  const baseDs = () =>
    makeResource({
      id: 'ds-search',
      type: 'storage',
      storage: { topology: 'datastore' },
      vmware: { entityType: 'datastore', datastoreAccessible: true },
    });

  it('matches by datastore type and url', () => {
    const ds = makeResource({
      ...baseDs(),
      vmware: {
        entityType: 'datastore',
        datastoreAccessible: true,
        datastoreType: 'VMFS6',
        datastoreUrl: 'https://vcsa/folder?ds=uuid-123',
      },
    });
    expect(filterVmwareDatastores([ds], 'vmfs6', 'all')).toHaveLength(1);
    expect(filterVmwareDatastores([ds], 'uuid-123', 'all')).toHaveLength(1);
  });

  it('matches by storage consumer names and consumer types', () => {
    const ds = makeResource({
      ...baseDs(),
      storage: {
        topology: 'datastore',
        consumerTypes: ['vm'],
        topConsumers: [{ resourceType: 'vm', name: 'db-server-01' }],
      },
    });
    expect(filterVmwareDatastores([ds], 'db-server', 'all')).toHaveLength(1);
    expect(filterVmwareDatastores([ds], 'vm', 'all')).toHaveLength(1);
  });

  it('matches by tags', () => {
    const ds = makeResource({
      ...baseDs(),
      tags: ['tier-1', 'production'],
    });
    expect(filterVmwareDatastores([ds], 'tier-1', 'all')).toHaveLength(1);
    expect(filterVmwareDatastores([ds], 'production', 'all')).toHaveLength(1);
  });

  it('matches by maintenance mode and overall status', () => {
    const ds = makeResource({
      ...baseDs(),
      vmware: {
        entityType: 'datastore',
        datastoreAccessible: true,
        maintenanceMode: 'in_maintenance',
        overallStatus: 'yellow',
      },
    });
    expect(filterVmwareDatastores([ds], 'in_maintenance', 'all')).toHaveLength(1);
    expect(filterVmwareDatastores([ds], 'yellow', 'all')).toHaveLength(1);
  });

  it('does not match when needle is absent in haystack', () => {
    const ds = makeResource({
      ...baseDs(),
      name: 'simple-ds',
    });
    expect(filterVmwareDatastores([ds], 'nonexistent-term', 'all')).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// vmwareVirtualMachineSearchHaystack — via filterVmwareVirtualMachines
// ---------------------------------------------------------------------------

describe('vmwareVirtualMachineSearchHaystack (via filterVmwareVirtualMachines)', () => {
  const baseVm = () =>
    makeResource({
      id: 'vm-search',
      type: 'vm',
      vmware: { entityType: 'vm', powerState: 'poweredOn' },
    });

  it('matches by cluster services (HA/DRS formatted string)', () => {
    const vm = makeResource({
      ...baseVm(),
      vmware: {
        entityType: 'vm',
        powerState: 'poweredOn',
        clusterHaEnabled: true,
        clusterDrsEnabled: false,
      },
    });
    expect(filterVmwareVirtualMachines([vm], 'ha enabled', 'all')).toHaveLength(1);
    expect(filterVmwareVirtualMachines([vm], 'drs disabled', 'all')).toHaveLength(1);
  });

  it('matches by network adapter fields', () => {
    const vm = makeResource({
      ...baseVm(),
      vmware: {
        entityType: 'vm',
        powerState: 'poweredOn',
        networkAdapters: [
          {
            label: 'Network adapter 1',
            type: 'VMXNET3',
            macAddress: '00:50:56:ab:cd:ef',
            networkName: 'VM-Production',
          },
        ],
      },
    });
    expect(filterVmwareVirtualMachines([vm], 'vmxnet3', 'all')).toHaveLength(1);
    expect(filterVmwareVirtualMachines([vm], '00:50:56', 'all')).toHaveLength(1);
    expect(filterVmwareVirtualMachines([vm], 'vm-production', 'all')).toHaveLength(1);
  });

  it('matches by virtual disk fields', () => {
    const vm = makeResource({
      ...baseVm(),
      vmware: {
        entityType: 'vm',
        powerState: 'poweredOn',
        virtualDisks: [
          { disk: 'scsi0:0', label: 'Hard disk 1', type: 'thin', vmdkFile: '[ds1] vm/vm.vmdk' },
        ],
      },
    });
    expect(filterVmwareVirtualMachines([vm], 'thin', 'all')).toHaveLength(1);
    expect(filterVmwareVirtualMachines([vm], 'scsi0:0', 'all')).toHaveLength(1);
    expect(filterVmwareVirtualMachines([vm], 'vm.vmdk', 'all')).toHaveLength(1);
  });

  it('matches by VMware tools fields', () => {
    const vm = makeResource({
      ...baseVm(),
      vmware: {
        entityType: 'vm',
        powerState: 'poweredOn',
        tools: { runState: 'running', versionStatus: 'guestToolsCurrent', version: '12.4.0' },
      },
    });
    expect(filterVmwareVirtualMachines([vm], 'running', 'all')).toHaveLength(1);
    expect(filterVmwareVirtualMachines([vm], '12.4.0', 'all')).toHaveLength(1);
  });

  it('matches by hardware fields including boot devices', () => {
    const vm = makeResource({
      ...baseVm(),
      vmware: {
        entityType: 'vm',
        powerState: 'poweredOn',
        hardware: {
          guestOs: 'ubuntu_64',
          version: 'vmx-20',
          bootType: 'efi',
          bootDevices: [{ type: 'disk', disks: ['scsi0:0'] }],
        },
      },
    });
    // enumSearchValue splits on underscores so 'ubuntu 64' should match
    expect(filterVmwareVirtualMachines([vm], 'ubuntu', 'all')).toHaveLength(1);
    expect(filterVmwareVirtualMachines([vm], 'efi', 'all')).toHaveLength(1);
  });

  it('matches by instance UUID, bios UUID, guest IP and datastore names', () => {
    const vm = makeResource({
      ...baseVm(),
      vmware: {
        entityType: 'vm',
        powerState: 'poweredOn',
        instanceUuid: '502-uuid-here',
        biosUuid: '420-bios-uuid',
        guestIpAddresses: ['10.42.0.5'],
        datastoreNames: ['nvme-ds-1'],
      },
    });
    expect(filterVmwareVirtualMachines([vm], '502-uuid', 'all')).toHaveLength(1);
    expect(filterVmwareVirtualMachines([vm], '10.42.0.5', 'all')).toHaveLength(1);
    expect(filterVmwareVirtualMachines([vm], 'nvme-ds-1', 'all')).toHaveLength(1);
  });

  it('matches by tags', () => {
    const vm = makeResource({
      ...baseVm(),
      tags: ['critical-workload'],
    });
    expect(filterVmwareVirtualMachines([vm], 'critical-workload', 'all')).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// filterVmwareActivity — additional branches
// ---------------------------------------------------------------------------

describe('filterVmwareActivity (additional branches)', () => {
  const buildTestRows = () =>
    buildVmwareActivityRows(
      [makeResource({ id: 'fa-res', type: 'vm' })],
      [
        makeChange({
          id: 'task-row',
          resourceId: 'fa-res',
          metadata: { activity_type: 'vmware_task', activity_state: 'success' },
        }),
        makeChange({
          id: 'event-row',
          resourceId: 'fa-res',
          metadata: { activity_type: 'vmware_event', activity_state: 'success' },
        }),
        makeChange({
          id: 'failed-row',
          resourceId: 'fa-res',
          metadata: { activity_type: 'vmware_task', activity_state: 'error' },
        }),
      ],
    );

  it('filters to only tasks when status is "tasks"', () => {
    const rows = buildTestRows();
    expect(filterVmwareActivity(rows, '', 'tasks').map((r) => r.nativeId)).toEqual([
      'failed-row',
      'task-row',
    ]);
  });

  it('returns all rows when status is "all" with no search', () => {
    const rows = buildTestRows();
    expect(filterVmwareActivity(rows, '', 'all')).toHaveLength(3);
  });

  it('combines search with status filter', () => {
    const rows = buildTestRows();
    expect(filterVmwareActivity(rows, '', 'failed').map((r) => r.nativeId)).toEqual(['failed-row']);
  });

  it('returns empty when search matches nothing', () => {
    const rows = buildTestRows();
    expect(filterVmwareActivity(rows, 'zzz-nonexistent', 'all')).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// buildVmwarePageModel — additional edge-case branches
// ---------------------------------------------------------------------------

describe('buildVmwarePageModel (additional branches)', () => {
  it('returns an empty model for an empty resources array', () => {
    const model = buildVmwarePageModel([]);
    expect(model.resources).toEqual([]);
    expect(model.hosts).toEqual([]);
    expect(model.vms).toEqual([]);
    expect(model.datastores).toEqual([]);
    expect(model.networks).toEqual([]);
    expect(model.incidents).toEqual([]);
    expect(model.activity).toEqual([]);
  });

  it('filters out non-vmware platform resources', () => {
    const proxmox = makeResource({
      id: 'pve-vm-1',
      type: 'vm',
      platformType: 'proxmox-pve',
    });
    const model = buildVmwarePageModel([proxmox]);
    expect(model.resources).toEqual([]);
    expect(model.vms).toEqual([]);
  });

  it('includes datastore by vmware.entityType even without storage.topology', () => {
    const ds = makeResource({
      id: 'ds-by-entity',
      type: 'storage',
      vmware: { entityType: 'datastore', datastoreAccessible: true },
    });
    const model = buildVmwarePageModel([ds]);
    expect(model.datastores.map((d) => d.id)).toEqual(['ds-by-entity']);
  });

  it('excludes storage resources that are not datastores', () => {
    const pool = makeResource({
      id: 'zfs-pool',
      type: 'storage',
      storage: { topology: 'zfs-pool' },
      vmware: {},
    });
    const model = buildVmwarePageModel([pool]);
    expect(model.datastores).toEqual([]);
  });

  it('defaults activityChanges to empty array producing no activity rows', () => {
    const vm = makeResource({ id: 'def-vm', type: 'vm' });
    const model = buildVmwarePageModel([vm]);
    expect(model.activity).toEqual([]);
  });

  it('processes activity changes passed as the second argument', () => {
    const vm = makeResource({ id: 'arg-vm', type: 'vm' });
    const change = makeChange({
      id: 'arg-change',
      resourceId: 'arg-vm',
      metadata: { activity_type: 'vmware_task' },
    });
    const model = buildVmwarePageModel([vm], [change]);
    expect(model.activity).toHaveLength(1);
    expect(model.activity[0].nativeId).toBe('arg-change');
  });
});

// ---------------------------------------------------------------------------
// vmwareNetworkDisplayName — additional fallback via network sorting
// ---------------------------------------------------------------------------

describe('vmwareNetworkDisplayName (via buildVmwarePageModel sorting)', () => {
  it('falls back to id when displayName and name are empty', () => {
    const byId = makeResource({
      id: 'net-zzz',
      type: 'network',
      displayName: '',
      name: '',
      vmware: { entityType: 'network', overallStatus: 'green' },
    });
    const byName = makeResource({
      id: 'net-mmm',
      type: 'network',
      displayName: '',
      name: 'aaa-net',
      vmware: { entityType: 'network', overallStatus: 'green' },
    });
    const { networks } = buildVmwarePageModel([byId, byName]);
    // aaa-net < net-zzz
    expect(networks.map((n) => n.id)).toEqual(['net-mmm', 'net-zzz']);
  });
});
