import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import type {
  ActionAuditRecord,
  ActionExecutionStatus,
  ActionVerificationTruthStatus,
} from '@/types/actionAudit';
import { getAPTActionPresentation } from '../aptActionPresentation';

const source = readFileSync(resolve(__dirname, '..', 'aptActionPresentation.ts'), 'utf8');

const makeAudit = ({
  capabilityName = 'install_os_updates',
  summary,
  execution = 'inconclusive',
  verification = 'inconclusive',
  reasonCode = 'capability_auto_elevated',
  params,
}: {
  capabilityName?: 'install_os_updates' | 'clean_package_cache';
  summary?: string;
  execution?: ActionExecutionStatus;
  verification?: ActionVerificationTruthStatus;
  reasonCode?: 'capability_auto_elevated' | 'capability_auto_low_risk';
  params?: Record<string, unknown>;
} = {}): ActionAuditRecord => ({
  id: 'apt-action',
  createdAt: '2026-07-12T10:00:00Z',
  updatedAt: '2026-07-12T10:01:00Z',
  state: 'completed',
  request: {
    requestId: 'apt-request',
    resourceId: 'proxmox:node:pve-1',
    capabilityName,
    params,
    reason: 'Resolve the current Patrol finding',
    requestedBy: 'pulse_patrol',
  },
  plan: {
    actionId: 'apt-action',
    requestId: 'apt-request',
    allowed: true,
    requiresApproval: reasonCode === 'capability_auto_elevated',
    approvalPolicy: reasonCode === 'capability_auto_elevated' ? 'admin' : 'none',
    rollbackAvailable: false,
    policyDecision: {
      version: 1,
      status: 'resolved',
      decisionId: 'decision',
      actionId: 'apt-action',
      scope: { orgId: 'org-1', resourceId: 'proxmox:node:pve-1', capabilityName },
      approvalRequirement: {
        version: 1,
        floor: reasonCode === 'capability_auto_elevated' ? 'admin' : 'none',
        quorum: 1,
        disallowRequester: false,
      },
      planningAllowed: true,
      requiresApproval: reasonCode === 'capability_auto_elevated',
      authorities: [
        {
          kind: 'capability_registry',
          sourceId: `capability-registry:${capabilityName}`,
          status: 'consulted',
          scope: { orgId: 'org-1', resourceId: 'proxmox:node:pve-1', capabilityName },
          reasonCodes: [reasonCode],
        },
      ],
    },
  },
  result: {
    success: execution === 'succeeded',
    actionResultV2: {
      version: 2,
      execution: { status: execution, summary },
      verification: { status: verification, evidenceClass: 'agent_attested' },
      compensation: { support: 'unavailable', status: 'not_available' },
    },
  },
  verificationOutcome: { status: 'unknown' },
});

describe('APT action presentation', () => {
  it('presents an elevated update plan with exact empty parameter authority', () => {
    const presentation = getAPTActionPresentation(makeAudit());
    expect(presentation).toMatchObject({
      safetyPosture: 'Elevated change',
      approvalPosture: 'Administrator approval required',
      parametersValid: true,
    });
    expect(presentation?.parameterAuthority).toContain(
      'accepts no command, path, package selection, removal choice, or reboot request',
    );
    expect(presentation?.authorityBoundary).toContain('cannot remove packages or reboot');
  });

  it('presents cleanup as low-risk eligible but irreversible and bounded', () => {
    const presentation = getAPTActionPresentation(
      makeAudit({ capabilityName: 'clean_package_cache', reasonCode: 'capability_auto_low_risk' }),
    );
    expect(presentation).toMatchObject({
      safetyPosture: 'Low-risk automation eligible',
      approvalPosture: 'No separate approval required by this plan',
    });
    expect(presentation?.authorityBoundary).toContain('only downloaded package data');
  });

  it('shows confirmed agent-attested update facts without inventing independent proof or reboot authority', () => {
    const presentation = getAPTActionPresentation(
      makeAudit({
        summary:
          'APT package updates: phase=complete; 6 pending before, 0 pending after; package manager health: healthy; recovery required: false; reboot required: true',
        execution: 'succeeded',
        verification: 'confirmed',
      }),
    );
    expect(presentation?.facts).toEqual(
      expect.arrayContaining([
        { label: 'Updates remaining', value: '0' },
        { label: 'Update system health', value: 'Known healthy' },
        { label: 'Reboot required', value: 'Yes — fact only; no reboot was authorized' },
      ]),
    );
    expect(presentation?.nextStep).toContain('separate governed action');
  });

  it('makes a partial unhealthy install recovery-required and forbids automatic retry', () => {
    const presentation = getAPTActionPresentation(
      makeAudit({
        summary:
          'APT package updates: phase=install; 6 pending before, 3 pending after; package manager health: unhealthy; recovery required: true; reboot required: false',
        execution: 'inconclusive',
        verification: 'contradicted',
      }),
    );
    expect(presentation?.facts).toEqual(
      expect.arrayContaining([
        { label: 'Last phase reached', value: 'Install updates' },
        { label: 'Updates remaining', value: '3' },
        { label: 'Update system health', value: 'Known unhealthy' },
        { label: 'Recovery required', value: 'Yes' },
      ]),
    );
    expect(presentation?.nextStep).toMatch(/^Do not retry\./);
  });

  it('keeps a delayed legacy result inconclusive when health is unknown', () => {
    const presentation = getAPTActionPresentation(
      makeAudit({
        summary:
          'APT package updates: phase=verify; 4 pending before, 4 pending after; package manager health: unknown; recovery required: false; reboot required: false',
      }),
    );
    expect(presentation?.facts).toContainEqual({ label: 'Update system health', value: 'Unknown' });
    expect(presentation?.nextStep).toContain('Do not retry automatically');
  });

  it('shows measured cleanup bytes and an irreversible rescan-required failure', () => {
    const presentation = getAPTActionPresentation(
      makeAudit({
        capabilityName: 'clean_package_cache',
        reasonCode: 'capability_auto_low_risk',
        summary:
          'APT package cache: phase=clean; 104857600 bytes before, 52428800 bytes after, 52428800 bytes reclaimed; rollback available: false; rescan required: true',
        execution: 'failed',
        verification: 'inconclusive',
      }),
    );
    expect(presentation?.facts).toEqual(
      expect.arrayContaining([
        { label: 'Downloaded package data before', value: '100 MB' },
        { label: 'Downloaded package data after', value: '50.0 MB' },
        { label: 'Space reclaimed', value: '50.0 MB' },
        { label: 'Rollback', value: 'Unavailable — cleanup is irreversible' },
        { label: 'Fresh rescan required', value: 'Yes' },
      ]),
    );
    expect(presentation?.nextStep).toContain('Do not retry automatically');
  });

  it('fails closed for unexpected parameters and malformed result summaries', () => {
    const presentation = getAPTActionPresentation(
      makeAudit({ params: { package: 'curl' }, summary: 'phase=complete; success=true' }),
    );
    expect(presentation?.parametersValid).toBe(false);
    expect(presentation?.facts).toEqual([]);
    expect(presentation?.parameterAuthority).toContain('Unexpected parameters');
    expect(presentation?.nextStep).toContain('Refresh this action record');
  });

  it.each([
    [
      'unknown update phase',
      'install_os_updates',
      'APT package updates: phase=download; 6 pending before, 3 pending after; package manager health: healthy; recovery required: false; reboot required: false',
    ],
    [
      'cleanup-only phase in update',
      'install_os_updates',
      'APT package updates: phase=clean; 6 pending before, 3 pending after; package manager health: healthy; recovery required: false; reboot required: false',
    ],
    [
      'update-only phase in cleanup',
      'clean_package_cache',
      'APT package cache: phase=install; 100 bytes before, 50 bytes after, 50 bytes reclaimed; rollback available: false; rescan required: false',
    ],
    [
      'unsafe update count',
      'install_os_updates',
      'APT package updates: phase=verify; 999999999999999999999 pending before, 0 pending after; package manager health: healthy; recovery required: false; reboot required: false',
    ],
    [
      'impossible cleanup remainder',
      'clean_package_cache',
      'APT package cache: phase=verify; 100 bytes before, 120 bytes after, 0 bytes reclaimed; rollback available: false; rescan required: true',
    ],
    [
      'impossible cleanup reclaimed arithmetic',
      'clean_package_cache',
      'APT package cache: phase=verify; 100 bytes before, 50 bytes after, 40 bytes reclaimed; rollback available: false; rescan required: true',
    ],
  ] as const)('fails closed for %s', (_name, capabilityName, summary) => {
    const presentation = getAPTActionPresentation(makeAudit({ capabilityName, summary }));
    expect(presentation?.facts).toEqual([]);
    expect(presentation?.nextStep).toContain('Refresh this action record');
  });

  it('does not declare a second frontend action truth dialect', () => {
    expect(source).not.toMatch(
      /(?:type|enum)\s+APT(?:Execution|Verification|Evidence|Health|Recovery|Compensation)/,
    );
    expect(source).toContain("import type { ActionAuditRecord } from '@/types/actionAudit'");
    expect(source).toContain('actionResultV2?.verification');
  });
});
