/**
 * Branch-coverage tests for the unexported helpers inside
 * infrastructureImportPlanModel. None of the target helpers
 * (coverageLabel, hostnameFromEndpoint, endpointFromCandidate,
 * credentialLabel, nodeTypeToMonitoredSource, nodeTypeToResourceType,
 * versionFromCandidate, candidateLabel) are exported, so every branch is
 * driven through the sole exported entry point `buildNodeImportPlan` and
 * asserted via the resulting plan fields.
 */
import { describe, expect, it } from 'vitest';
import type { ProbeCandidate } from '@/api/connections';
import {
  getNodeModalDefaultFormData,
  type NodeModalFormData,
  type NodeModalNodeType,
} from '@/utils/nodeModalPresentation';
import type { DiscoveredServer } from '../infrastructureSettingsModel';
import {
  buildNodeImportPlan,
  type NodeImportPlan,
  type NodeImportPlanCandidate,
} from '../infrastructureImportPlanModel';

// ---- Fixtures ---------------------------------------------------------------

const makeServer = (overrides: Partial<DiscoveredServer> = {}): DiscoveredServer => ({
  type: 'pve',
  ip: '10.0.0.10',
  hostname: 'tower.local',
  port: 8006,
  version: 'Proxmox VE',
  release: '9.0',
  ...overrides,
});

const makeProbe = (overrides: Partial<ProbeCandidate> = {}): ProbeCandidate => ({
  type: 'pve',
  host: 'https://probe.local:8006',
  port: 8006,
  hints: { product: 'Proxmox VE', version: '9.0' },
  ...overrides,
});

const makeForm = (
  nodeType: NodeModalNodeType,
  overrides: Partial<NodeModalFormData> = {},
): NodeModalFormData => ({
  ...getNodeModalDefaultFormData(nodeType),
  ...overrides,
});

const discoveryCandidate = (server: DiscoveredServer): NodeImportPlanCandidate => ({
  kind: 'discovery',
  server,
});

const probeCandidate = (candidate: ProbeCandidate): NodeImportPlanCandidate => ({
  kind: 'probe',
  candidate,
});

// ---- nodeTypeToMonitoredSource + nodeTypeToResourceType --------------------
// pve -> 'proxmox'/'agent' and pbs -> 'pbs'/'pbs' are exercised by the sibling
// test file. Here we cover the pmg arms of both switches.

describe('nodeTypeToMonitoredSource / nodeTypeToResourceType (pmg arm)', () => {
  it("maps pmg to the 'pmg' monitored source and resource type", () => {
    const plan = buildNodeImportPlan(
      'pmg',
      makeForm('pmg', { name: 'mail', host: 'https://mail.local:8006' }),
      discoveryCandidate(makeServer({ type: 'pmg', hostname: 'mail.local', port: 8006 })),
    );

    expect(plan).not.toBeNull();
    expect(plan?.source).toBe('pmg');
    expect(plan?.previewRequest?.candidate.type).toBe('pmg');
  });
});

// ---- hostnameFromEndpoint ---------------------------------------------------

describe('hostnameFromEndpoint (via plan.hostname)', () => {
  it('returns an empty hostname when the endpoint is empty', () => {
    // formData.host empty AND probe host empty => endpoint '' => hostname ''.
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: '' }),
      probeCandidate(makeProbe({ host: '' })),
    );

    expect(plan?.endpoint).toBe('');
    expect(plan?.hostname).toBe('');
  });

  it('parses a scheme-less host by implicitly prepending https://', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'tower.local:8006' }),
      probeCandidate(makeProbe()),
    );

    // formData.host wins over candidate; endpoint keeps the scheme-less form.
    expect(plan?.endpoint).toBe('tower.local:8006');
    expect(plan?.hostname).toBe('tower.local');
  });

  it('falls back to manual host stripping when the URL cannot be parsed', () => {
    // 'bad:host' makes `new URL('https://bad:host')` throw (invalid port); the
    // catch arm strips any scheme, drops the path, then drops the port.
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'bad:host' }),
      probeCandidate(makeProbe()),
    );

    expect(plan?.hostname).toBe('bad');
  });
});

// ---- endpointFromCandidate --------------------------------------------------
// Only reached when formData.host is empty/whitespace, because
// `normalizeEndpoint(formData.host) || endpointFromCandidate(candidate)`.

describe('endpointFromCandidate (via plan.endpoint with empty formData.host)', () => {
  it('normalizes (trims) the probe candidate host', () => {
    const plan = buildNodeImportPlan(
      'pbs',
      makeForm('pbs', { host: '' }),
      probeCandidate(
        makeProbe({ type: 'pbs', host: '   https://backup.local:8007   ', port: 8007 }),
      ),
    );

    expect(plan?.endpoint).toBe('https://backup.local:8007');
  });

  it('prefers the discovery server hostname when present', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { host: '' }),
      discoveryCandidate(makeServer({ hostname: 'host.local', ip: '10.0.0.20', port: 8006 })),
    );

    expect(plan?.endpoint).toBe('https://host.local:8006');
  });

  it('falls back to the IP when the discovery hostname is an empty string', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { host: '' }),
      discoveryCandidate(makeServer({ hostname: '', ip: '10.0.0.20', port: 8006 })),
    );

    expect(plan?.endpoint).toBe('https://10.0.0.20:8006');
  });

  it('falls back to the IP when the discovery hostname is undefined', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { host: '' }),
      discoveryCandidate(makeServer({ hostname: undefined, ip: '10.0.0.20', port: 8006 })),
    );

    expect(plan?.endpoint).toBe('https://10.0.0.20:8006');
  });
});

// ---- versionFromCandidate ---------------------------------------------------

describe('versionFromCandidate (via plan.detectedVersion)', () => {
  it('joins product and version hints for a probe candidate', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'https://x.local' }),
      probeCandidate(makeProbe({ hints: { product: 'Proxmox VE', version: '9.1' } })),
    );

    expect(plan?.detectedVersion).toBe('Proxmox VE 9.1');
  });

  it('returns only the product when the version hint is absent', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'https://x.local' }),
      probeCandidate(makeProbe({ hints: { product: 'Proxmox VE' } })),
    );

    expect(plan?.detectedVersion).toBe('Proxmox VE');
  });

  it('returns only the version when the product hint is absent', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'https://x.local' }),
      probeCandidate(makeProbe({ hints: { version: '9.1' } })),
    );

    expect(plan?.detectedVersion).toBe('9.1');
  });

  it('uses the generic fallback when hints are missing entirely', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'https://x.local' }),
      probeCandidate(makeProbe({ hints: undefined })),
    );

    expect(plan?.detectedVersion).toBe('Detected API endpoint');
  });

  it('uses the generic fallback when both hints trim to empty', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'https://x.local' }),
      probeCandidate(makeProbe({ hints: { product: '   ', version: '' } })),
    );

    expect(plan?.detectedVersion).toBe('Detected API endpoint');
  });

  it('joins server version and release for a discovery candidate', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'https://x.local' }),
      discoveryCandidate(makeServer({ version: 'Proxmox VE', release: '8.4' })),
    );

    expect(plan?.detectedVersion).toBe('Proxmox VE 8.4');
  });

  it('returns only the server version when release is undefined', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'https://x.local' }),
      discoveryCandidate(makeServer({ version: 'Proxmox VE', release: undefined })),
    );

    expect(plan?.detectedVersion).toBe('Proxmox VE');
  });

  it('returns only the release when version is empty', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'https://x.local' }),
      discoveryCandidate(makeServer({ version: '', release: '9.0' })),
    );

    expect(plan?.detectedVersion).toBe('9.0');
  });

  it('returns an empty detected version when both version and release are empty', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'https://x.local' }),
      discoveryCandidate(makeServer({ version: '', release: '' })),
    );

    expect(plan?.detectedVersion).toBe('');
  });
});

// ---- candidateLabel ---------------------------------------------------------

describe('candidateLabel (via plan.candidateLabel and steps[0].detail)', () => {
  it('describes a probe candidate with its host', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'https://x.local' }),
      probeCandidate(makeProbe({ host: 'https://detected.local:8006' })),
    );

    expect(plan?.candidateLabel).toBe('Detected API probe at https://detected.local:8006');
    // buildSteps feeds the candidate summary into the first step detail.
    expect(plan?.steps[0]?.detail).toBe(plan?.candidateLabel);
  });

  it('describes a discovery candidate using its hostname', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'https://x.local' }),
      discoveryCandidate(makeServer({ hostname: 'node7.local', ip: '10.0.0.5', port: 8006 })),
    );

    expect(plan?.candidateLabel).toBe('Discovery candidate at node7.local:8006');
  });

  it('falls back to the IP in the discovery label when hostname is empty', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'https://x.local' }),
      discoveryCandidate(makeServer({ hostname: '', ip: '10.0.0.5', port: 8006 })),
    );

    expect(plan?.candidateLabel).toBe('Discovery candidate at 10.0.0.5:8006');
  });

  it('falls back to the IP in the discovery label when hostname is undefined', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'https://x.local' }),
      discoveryCandidate(makeServer({ hostname: undefined, ip: '10.0.0.5', port: 8006 })),
    );

    expect(plan?.candidateLabel).toBe('Discovery candidate at 10.0.0.5:8006');
  });
});

// ---- credentialLabel --------------------------------------------------------

describe('credentialLabel (via plan.credentialLabel and steps[1].detail)', () => {
  it('emits the password-validation message when authType is password', () => {
    // authType === 'password' short-circuits ahead of setupMode.
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', {
        name: 'x',
        host: 'https://x.local',
        authType: 'password',
        setupMode: 'auto',
      }),
      probeCandidate(makeProbe()),
    );

    expect(plan?.credentialLabel).toBe('Validate username and password credentials before saving.');
    expect(plan?.steps[1]?.detail).toBe(plan?.credentialLabel);
  });

  it('emits the API-token message for token + manual setup', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', {
        name: 'x',
        host: 'https://x.local',
        authType: 'token',
        setupMode: 'manual',
      }),
      probeCandidate(makeProbe()),
    );

    expect(plan?.credentialLabel).toBe('Validate the supplied API token before saving.');
  });

  it('emits the agent handoff message for token + agent setup', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', {
        name: 'x',
        host: 'https://x.local',
        authType: 'token',
        setupMode: 'agent',
      }),
      probeCandidate(makeProbe()),
    );

    expect(plan?.credentialLabel).toBe(
      'Run the Host Telemetry Agent handoff; Pulse will add the source after the agent reports.',
    );
  });

  it('emits the inventory-setup fallback for token + auto setup', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'https://x.local', authType: 'token', setupMode: 'auto' }),
      probeCandidate(makeProbe()),
    );

    expect(plan?.credentialLabel).toBe(
      'Run the API Inventory setup handoff; Pulse will add the source after setup completes.',
    );
  });
});

// ---- coverageLabel ----------------------------------------------------------

describe('coverageLabel (via plan.coverageLabel)', () => {
  it('lists only the SMART-disks collector when it is the sole enabled PVE collector', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', {
        name: 'x',
        host: 'https://x.local',
        monitorVMs: false,
        monitorContainers: false,
        monitorStorage: false,
        monitorBackups: false,
        monitorPhysicalDisks: true,
      }),
      probeCandidate(makeProbe()),
    );

    expect(plan?.coverageLabel).toBe('SMART disks');
  });

  it('joins a selected subset of PVE collectors in declared order', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', {
        name: 'x',
        host: 'https://x.local',
        monitorVMs: true,
        monitorContainers: false,
        monitorStorage: true,
        monitorBackups: false,
        monitorPhysicalDisks: false,
      }),
      probeCandidate(makeProbe()),
    );

    expect(plan?.coverageLabel).toBe('VMs, storage');
  });

  it('reports no Proxmox collectors when every PVE toggle is off', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', {
        name: 'x',
        host: 'https://x.local',
        monitorVMs: false,
        monitorContainers: false,
        monitorStorage: false,
        monitorBackups: false,
        monitorPhysicalDisks: false,
      }),
      probeCandidate(makeProbe()),
    );

    expect(plan?.coverageLabel).toBe('No Proxmox collectors selected');
  });

  it('reports no PBS collectors when every PBS toggle is off', () => {
    const plan = buildNodeImportPlan(
      'pbs',
      makeForm('pbs', {
        name: 'x',
        host: 'https://x.local',
        monitorDatastores: false,
        monitorSyncJobs: false,
        monitorVerifyJobs: false,
        monitorPruneJobs: false,
        monitorGarbageJobs: false,
      }),
      probeCandidate(makeProbe({ type: 'pbs' })),
    );

    expect(plan?.coverageLabel).toBe('No PBS collectors selected');
  });

  it('lists the default PMG collectors in declared order', () => {
    const plan = buildNodeImportPlan(
      'pmg',
      makeForm('pmg', { name: 'x', host: 'https://x.local' }),
      probeCandidate(makeProbe({ type: 'pmg' })),
    );

    // default PMG form: mail stats + queues + quarantine on, domain stats off.
    expect(plan?.coverageLabel).toBe('mail stats, queues, quarantine');
  });

  it('reports no PMG collectors when every PMG toggle is off', () => {
    const plan = buildNodeImportPlan(
      'pmg',
      makeForm('pmg', {
        name: 'x',
        host: 'https://x.local',
        monitorMailStats: false,
        monitorQueues: false,
        monitorQuarantine: false,
        monitorDomainStats: false,
      }),
      probeCandidate(makeProbe({ type: 'pmg' })),
    );

    expect(plan?.coverageLabel).toBe('No PMG collectors selected');
  });
});

// ---- buildNodeImportPlan guards + wiring -----------------------------------

describe('buildNodeImportPlan guards and wiring', () => {
  it('returns null when the candidate is null', () => {
    const plan = buildNodeImportPlan('pve', makeForm('pve'), null);

    expect(plan).toBeNull();
  });

  it('returns null when the candidate is undefined', () => {
    const plan = buildNodeImportPlan('pve', makeForm('pve'), undefined);

    expect(plan).toBeNull();
  });

  it('omits the preview request when both endpoint and derived name are empty', () => {
    // Empty formData.host + empty probe host => endpoint '' => hostname ''.
    // Empty name => deriveNameFromHost('') === '' => name ''.
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: '', host: '' }),
      probeCandidate(makeProbe({ host: '' })),
    ) as NodeImportPlan;

    expect(plan.previewRequest).toBeNull();
  });

  it('derives the node name from the endpoint host when formData.name is empty', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: '', host: 'https://node7.local:8006' }),
      probeCandidate(makeProbe()),
    );

    expect(plan?.name).toBe('node7.local');
    // name + endpoint both truthy => preview request is populated.
    expect(plan?.previewRequest?.candidate.name).toBe('node7.local');
    expect(plan?.previewRequest?.candidate.resource_id).toBe('node7.local');
  });

  it('composes the signature from source, candidate, version, name, endpoint, auth, setup, coverage', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', {
        name: 'tower',
        host: 'https://tower.local:8006',
        authType: 'token',
        setupMode: 'manual',
        monitorVMs: true,
        monitorContainers: false,
        monitorStorage: false,
        monitorBackups: false,
        monitorPhysicalDisks: false,
      }),
      discoveryCandidate(makeServer({ version: 'Proxmox VE', release: '9.0' })),
    );

    expect(plan?.signature).toBe(
      [
        'proxmox',
        'Discovery candidate at tower.local:8006',
        'Proxmox VE 9.0',
        'tower',
        'https://tower.local:8006',
        'token',
        'manual',
        'VMs',
      ].join('|'),
    );
  });

  it('emits the five canonical steps in order', () => {
    const plan = buildNodeImportPlan(
      'pve',
      makeForm('pve', { name: 'x', host: 'https://x.local' }),
      probeCandidate(makeProbe()),
    ) as NodeImportPlan;

    expect(plan.steps.map((s) => s.id)).toEqual([
      'candidate',
      'credentials',
      'dry-run',
      'approval',
      'verification',
    ]);
    // The dry-run/approval/verification details are static strings.
    expect(plan.steps[2]?.detail).toBe(
      'Preview whether this source creates a new monitored system or attaches to an existing one.',
    );
  });
});
