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
