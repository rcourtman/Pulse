import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { cleanup, render, screen, within } from '@solidjs/testing-library';

import { ResourceActionHistory } from '../ResourceActionHistory';
import type { ActionAuditRecord } from '@/types/actionAudit';

const sourceText = readFileSync(resolve(__dirname, '..', 'ResourceActionHistory.tsx'), 'utf-8');

describe('ResourceActionHistory verification rendering', () => {
  it('renders the post-dispatch verification outcome on each audit row when ran=true', () => {
    // The broker's read-after-write verification outcome lives on
    // result.verification (ActionVerificationResult). The audit history row
    // must surface it without implying independent evidence. Pin the wiring
    // so the surface cannot silently regress to an output-only render.
    expect(sourceText).toContain('shouldRenderActionAuditVerification(props.audit)');
    expect(sourceText).toContain('Legacy check passed (source unclassified)');
    expect(sourceText).toContain('Legacy check failed (source unclassified)');
  });

  it('shows the verification command and output verbatim when present', () => {
    // The verification command (e.g. "systemctl is-active 'nginx'") is
    // operator-trusted context, not just internal plumbing — it must be
    // rendered so the operator can see exactly what Pulse read back, not
    // just a yes/no.
    expect(sourceText).toContain('v.command');
    expect(sourceText).toContain('v.output');
    expect(sourceText).toContain('v.note');
  });

  it('renders exactly one verification row for ran=true and omits ran=false', () => {
    cleanup();
    render(() =>
      ResourceActionHistory({
        audits: [
          actionAudit({
            id: 'action-verified',
            verification: {
              ran: true,
              success: true,
              command: "systemctl is-active 'nginx'",
              output: 'active',
              note: 'service reported active',
            },
          }),
          actionAudit({
            id: 'action-unverified',
            request: {
              requestId: 'req-action-unverified',
              resourceId: 'vm:42',
              capabilityName: 'restart_service',
              reason: 'Ran without a derivable verifier',
              requestedBy: 'agent:ops',
            },
            verification: {
              ran: false,
              success: false,
              command: 'should not render',
              output: 'sensitive output',
              note: 'should not render',
            },
          }),
        ],
        count: 2,
        loadingLabel: 'Actions loaded',
        error: '',
        onRetry: () => undefined,
      }),
    );

    const actionHistory = within(screen.getByTestId('resource-action-history-section'));
    expect(actionHistory.getAllByText('Legacy check passed (source unclassified)')).toHaveLength(1);
    expect(actionHistory.getByText("systemctl is-active 'nginx'")).toBeInTheDocument();
    expect(actionHistory.queryByText('should not render')).toBeNull();
    expect(actionHistory.queryByText('sensitive output')).toBeNull();
  });

  it('keeps APT execution, verification, recovery, bounded facts, and next step separate without a duplicate legacy card', () => {
    cleanup();
    const apt = actionAudit({
      request: {
        requestId: 'apt-request',
        resourceId: 'proxmox:node:pve-1',
        capabilityName: 'install_os_updates',
        reason: 'Resolve pending operating system updates',
        requestedBy: 'pulse_patrol',
      },
      plan: {
        actionId: 'apt-action',
        requestId: 'apt-request',
        allowed: true,
        requiresApproval: true,
        approvalPolicy: 'admin',
        rollbackAvailable: false,
      },
      result: {
        success: false,
        actionResultV2: {
          version: 2,
          execution: {
            status: 'inconclusive',
            summary:
              'APT package updates: phase=install; 6 pending before, 3 pending after; package manager health: unhealthy; recovery required: true; reboot required: false',
          },
          verification: {
            status: 'contradicted',
            evidenceClass: 'agent_attested',
            summary: 'Three updates still remain.',
          },
          compensation: {
            support: 'unavailable',
            status: 'not_available',
            summary: 'No rollback is available; repair is required.',
          },
        },
      },
      verification: {
        ran: true,
        success: false,
        command: 'legacy command must not render',
        output: 'legacy output must not render',
      },
    });
    render(() =>
      ResourceActionHistory({
        audits: [apt],
        count: 1,
        loadingLabel: 'Actions loaded',
        error: '',
        onRetry: () => undefined,
      }),
    );
    const history = within(screen.getByTestId('resource-action-history-section'));
    expect(history.getByText('Execution inconclusive')).toBeInTheDocument();
    expect(history.getByText('Outcome contradicted')).toBeInTheDocument();
    expect(history.getByText('Recovery: Not Available')).toBeInTheDocument();
    expect(history.getByText('Known unhealthy')).toBeInTheDocument();
    expect(
      history.getByText(
        'Do not retry. Repair the host update system, run a fresh scan, and create a new plan only after health is known.',
      ),
    ).toBeInTheDocument();
    expect(history.queryByText('legacy command must not render')).toBeNull();
    expect(history.queryByText('Legacy check failed (source unclassified)')).toBeNull();
    expect(screen.getAllByTestId('resource-action-recovery-truth')).toHaveLength(1);
  });
});

const actionAudit = (overrides: Partial<ActionAuditRecord> = {}): ActionAuditRecord => ({
  id: overrides.id ?? 'action-1',
  createdAt: '2026-04-29T12:00:00Z',
  updatedAt: '2026-04-29T12:01:00Z',
  state: 'completed',
  request: overrides.request ?? {
    requestId: 'req-1',
    resourceId: 'vm:42',
    capabilityName: 'restart_service',
    reason: 'Restart nginx after patching',
    requestedBy: 'agent:ops',
  },
  plan: overrides.plan ?? {
    actionId: overrides.id ?? 'action-1',
    requestId: 'req-1',
    allowed: true,
    requiresApproval: true,
    approvalPolicy: 'admin',
    rollbackAvailable: true,
  },
  result: overrides.result ?? {
    success: true,
    output: 'completed',
  },
  verification: overrides.verification,
  approvals: overrides.approvals,
  verificationOutcome: overrides.verificationOutcome,
});
