import { describe, expect, it } from 'vitest';

import type {
  ActionExecutionTruth,
  ActionResultV2,
  ActionVerificationTruth,
} from '@/types/actionAudit';
import {
  getActionAuditResultPresentation,
  getActionAuditVerificationOutcomePresentation,
} from '@/utils/actionAuditPresentation';

// This suite is scoped to the two named presentation functions and targets the
// `actionResultV2` "truth-layer" branches that the sibling suites never reach:
// every existing test for these two functions exercises either the legacy
// `verificationOutcome` map or the pre-v2 `result` shape, so the v2 execution
// and verification truth branches below were entirely uncovered.

const auditWithExecutionTruth = (
  execution: ActionExecutionTruth,
): Parameters<typeof getActionAuditResultPresentation>[0] => ({
  // success:true forces getActionAuditRefusalPresentation's
  // `!audit.result || audit.result.success` guard to short-circuit, so the
  // refusal branch is skipped and the execution-truth branch is reached.
  result: {
    success: true,
    actionResultV2: {
      version: 2,
      execution,
      verification: { status: 'not_attempted', evidenceClass: 'none' },
      compensation: { support: 'unavailable', status: 'not_needed' },
    },
  },
});

const auditWithVerificationTruth = (
  verification: ActionVerificationTruth,
): Parameters<typeof getActionAuditVerificationOutcomePresentation>[0] => ({
  result: {
    success: true,
    actionResultV2: {
      version: 2,
      execution: { status: 'succeeded' },
      verification,
      compensation: { support: 'unavailable', status: 'not_needed' },
    },
  },
});

describe('getActionAuditResultPresentation — actionResultV2 execution truth path', () => {
  it('maps a succeeded execution to the success badge, trimming summary and formatting reasonCode (both truthy ternary arms)', () => {
    const presentation = getActionAuditResultPresentation(
      auditWithExecutionTruth({
        status: 'succeeded',
        summary: '  service restarted  ',
        reasonCode: 'executor_timeout',
      }),
    );
    expect(presentation).toStrictEqual({
      kind: 'success',
      label: 'Execution succeeded',
      detail: 'service restarted',
      reasonLabel: 'Executor Timeout',
      className:
        'border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-200',
    });
  });

  it('maps a failed execution with no summary/reasonCode to the failure badge (both falsy ternary arms)', () => {
    const presentation = getActionAuditResultPresentation(
      auditWithExecutionTruth({ status: 'failed' }),
    );
    expect(presentation).toStrictEqual({
      kind: 'failure',
      label: 'Execution failed',
      detail: undefined,
      reasonLabel: undefined,
      className:
        'border-red-200 bg-red-50 text-red-800 dark:border-red-800 dark:bg-red-950/40 dark:text-red-300',
    });
  });

  it('maps a not_run execution to the neutral "did not run" badge and collapses whitespace summary to undefined', () => {
    // `truth.summary?.trim()` -> '' -> `|| undefined` (trim-to-empty falsy arm).
    const presentation = getActionAuditResultPresentation(
      auditWithExecutionTruth({ status: 'not_run', summary: '   ' }),
    );
    expect(presentation).toStrictEqual({
      kind: 'failure',
      label: 'Execution did not run',
      detail: undefined,
      reasonLabel: undefined,
      className: 'border-border bg-surface text-base-content',
    });
  });

  it('maps an inconclusive execution to the amber badge with a formatted reasonLabel', () => {
    const presentation = getActionAuditResultPresentation(
      auditWithExecutionTruth({
        status: 'inconclusive',
        summary: 'partial output captured',
        reasonCode: 'window_closed',
      }),
    );
    expect(presentation).toStrictEqual({
      kind: 'failure',
      label: 'Execution inconclusive',
      detail: 'partial output captured',
      reasonLabel: 'Window Closed',
      className:
        'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300',
    });
  });

  it('falls through to the success branch when actionResultV2 exists but execution is absent (optional-chain short-circuit)', () => {
    // `result.actionResultV2?.execution` -> undefined when the execution key is
    // missing, so `if (truth)` is false and control reaches the `result.success`
    // branch. Distinct from actionResultV2 being entirely absent (covered by
    // sibling suites): here actionResultV2 is present, exercising the `?.`
    // short-circuit on a real-but-incomplete v2 record.
    const presentation = getActionAuditResultPresentation({
      result: {
        success: true,
        actionResultV2: {
          version: 2,
          verification: { status: 'not_attempted', evidenceClass: 'none' },
          compensation: { support: 'unavailable', status: 'not_needed' },
        } as unknown as ActionResultV2,
      },
    });
    expect(presentation).toStrictEqual({
      kind: 'success',
      label: 'Result',
      detail: undefined,
      className: 'border-border bg-surface text-base-content',
    });
  });
});

describe('getActionAuditVerificationOutcomePresentation — actionResultV2 verification truth path', () => {
  it('renders confirmed+independent with the independent-observer copy, trimmed summary, and emerald badge', () => {
    const presentation = getActionAuditVerificationOutcomePresentation(
      auditWithVerificationTruth({
        status: 'confirmed',
        evidenceClass: 'independent',
        summary: '  readback matched  ',
      }),
    );
    expect(presentation).toStrictEqual({
      label: 'Confirmed by independent observer',
      detail: 'An observer in a different trust domain confirmed the intended state.',
      evidenceSummary: 'readback matched Source: Independent observer.',
      className:
        'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300',
    });
  });

  it('renders confirmed+agent_attested with the agent-attested copy and the default summary fallback', () => {
    // Exercises the agent_attested arm of both the `source` ternary and the
    // nested `confirmed` statusCopy ternary, plus the `summary?.trim() ||`
    // fallback arm (no summary -> canned message).
    const presentation = getActionAuditVerificationOutcomePresentation(
      auditWithVerificationTruth({
        status: 'confirmed',
        evidenceClass: 'agent_attested',
      }),
    );
    expect(presentation).toStrictEqual({
      label: 'Confirmed by executing agent',
      detail: 'The same agent trust domain that executed the action reported the intended state.',
      evidenceSummary:
        'No additional verification summary. Source: Executing agent (agent-attested).',
      className:
        'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300',
    });
  });

  it('renders confirmed+none with the "lacks an evidence source" copy (else arm of both evidenceClass ternaries)', () => {
    const presentation = getActionAuditVerificationOutcomePresentation(
      auditWithVerificationTruth({
        status: 'confirmed',
        evidenceClass: 'none',
      }),
    );
    expect(presentation).toStrictEqual({
      label: 'Confirmation lacks an evidence source',
      detail:
        'The record says confirmed but provides no evidence source; do not treat it as independently verified.',
      evidenceSummary: 'No additional verification summary. Source: No evidence source.',
      className:
        'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300',
    });
  });

  it('renders contradicted with the rose badge (contradicted statusCopy arm + rose className arm)', () => {
    const presentation = getActionAuditVerificationOutcomePresentation(
      auditWithVerificationTruth({
        status: 'contradicted',
        evidenceClass: 'independent',
        summary: 'observed state differed',
      }),
    );
    expect(presentation).toStrictEqual({
      label: 'Outcome contradicted',
      detail: 'Observed evidence contradicted the intended state.',
      evidenceSummary: 'observed state differed Source: Independent observer.',
      className:
        'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-300',
    });
  });

  it('renders inconclusive with the amber badge (inconclusive statusCopy arm + amber className else arm)', () => {
    const presentation = getActionAuditVerificationOutcomePresentation(
      auditWithVerificationTruth({
        status: 'inconclusive',
        evidenceClass: 'agent_attested',
      }),
    );
    expect(presentation).toStrictEqual({
      label: 'Outcome inconclusive',
      detail: 'Available evidence could not establish the intended state.',
      evidenceSummary:
        'No additional verification summary. Source: Executing agent (agent-attested).',
      className:
        'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300',
    });
  });

  it('renders not_attempted with the amber badge (not_attempted statusCopy arm + amber className else arm)', () => {
    const presentation = getActionAuditVerificationOutcomePresentation(
      auditWithVerificationTruth({
        status: 'not_attempted',
        evidenceClass: 'none',
      }),
    );
    expect(presentation).toStrictEqual({
      label: 'Outcome not verified',
      detail: 'No outcome verification was attempted.',
      evidenceSummary: 'No additional verification summary. Source: No evidence source.',
      className:
        'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300',
    });
  });

  it('falls through to the legacy verificationOutcome path when actionResultV2 exists but verification is absent', () => {
    // `audit.result?.actionResultV2?.verification` -> undefined when the
    // verification key is missing, so `if (truth)` is false and control reaches
    // the `verificationOutcome` legacy branch. Distinct from result/actionResultV2
    // being absent: here actionResultV2 is present, exercising the `?.verification`
    // short-circuit on a real-but-incomplete v2 record.
    const presentation = getActionAuditVerificationOutcomePresentation({
      result: {
        success: true,
        actionResultV2: {
          version: 2,
          execution: { status: 'succeeded' },
          compensation: { support: 'unavailable', status: 'not_needed' },
        } as unknown as ActionResultV2,
      },
      verificationOutcome: {
        status: 'verified',
        evidenceSummary: 'legacy readback',
      },
    });
    expect(presentation).toStrictEqual({
      label: 'Legacy check passed (source unclassified)',
      detail:
        'This older record does not identify an evidence source. Do not treat it as independent confirmation.',
      evidenceSummary: 'legacy readback',
      className:
        'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300',
    });
  });
});
