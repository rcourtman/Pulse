import type { ActionAuditRecord } from '@/types/actionAudit';
import { formatBytes } from '@/utils/format';

export interface APTActionPresentationFact {
  label: string;
  value: string;
}

export interface APTActionPresentation {
  title: string;
  safetyPosture: string;
  approvalPosture: string;
  parameterAuthority: string;
  parametersValid: boolean;
  authorityBoundary: string;
  facts: APTActionPresentationFact[];
  nextStep: string;
}

const parseCount = (value: string): number | undefined => {
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed >= 0 ? parsed : undefined;
};

const formatPhase = (value: string, workflow: 'update' | 'cleanup'): string | undefined => {
  switch (value.trim()) {
    case 'preflight':
      return 'Safety check before changes';
    case 'refresh':
      return workflow === 'update' ? 'Refresh update information' : undefined;
    case 'install':
      return workflow === 'update' ? 'Install updates' : undefined;
    case 'clean':
      return workflow === 'cleanup' ? 'Clear downloaded package data' : undefined;
    case 'verify':
      return 'Check the result';
    case 'complete':
      return 'Completed workflow';
    default:
      return undefined;
  }
};

const getPolicyReasonCodes = (audit: ActionAuditRecord): string[] =>
  (audit.plan.policyDecision?.authorities ?? []).flatMap((authority) => authority.reasonCodes);

const hasExactEmptyParameters = (audit: ActionAuditRecord): boolean =>
  audit.request.params === undefined || Object.keys(audit.request.params).length === 0;

const getUpdateFacts = (summary: string): APTActionPresentationFact[] => {
  const match = summary.match(
    /^APT package updates: phase=([^;]+); (\d+) pending before, (\d+) pending after; package manager health: (healthy|unhealthy|unknown); recovery required: (true|false); reboot required: (true|false)$/,
  );
  if (!match) return [];
  const before = parseCount(match[2]);
  const after = parseCount(match[3]);
  const phase = formatPhase(match[1], 'update');
  if (before === undefined || after === undefined || !phase) return [];
  const health = match[4] === 'healthy' ? 'Known healthy' : match[4] === 'unhealthy' ? 'Known unhealthy' : 'Unknown';
  return [
    { label: 'Last phase reached', value: phase },
    { label: 'Updates before', value: String(before) },
    { label: 'Updates remaining', value: String(after) },
    { label: 'Update system health', value: health },
    { label: 'Recovery required', value: match[5] === 'true' ? 'Yes' : 'No' },
    { label: 'Reboot required', value: match[6] === 'true' ? 'Yes — fact only; no reboot was authorized' : 'No' },
  ];
};

const getCleanupFacts = (summary: string): APTActionPresentationFact[] => {
  const match = summary.match(
    /^APT package cache: phase=([^;]+); (\d+) bytes before, (\d+) bytes after, (\d+) bytes reclaimed; rollback available: false; rescan required: (true|false)$/,
  );
  if (!match) return [];
  const before = parseCount(match[2]);
  const after = parseCount(match[3]);
  const reclaimed = parseCount(match[4]);
  const phase = formatPhase(match[1], 'cleanup');
  if (
    before === undefined ||
    after === undefined ||
    reclaimed === undefined ||
    !phase ||
    after > before ||
    reclaimed !== before - after
  ) return [];
  return [
    { label: 'Last phase reached', value: phase },
    { label: 'Downloaded package data before', value: formatBytes(before) },
    { label: 'Downloaded package data after', value: formatBytes(after) },
    { label: 'Space reclaimed', value: formatBytes(reclaimed) },
    { label: 'Rollback', value: 'Unavailable — cleanup is irreversible' },
    { label: 'Fresh rescan required', value: match[5] === 'true' ? 'Yes' : 'No' },
  ];
};

const getFactValue = (facts: APTActionPresentationFact[], label: string): string =>
  facts.find((fact) => fact.label === label)?.value ?? '';

export const isAPTAction = (audit: Pick<ActionAuditRecord, 'request'>): boolean =>
  audit.request.capabilityName === 'install_os_updates' ||
  audit.request.capabilityName === 'clean_package_cache';

export const getAPTActionPresentation = (
  audit: ActionAuditRecord,
): APTActionPresentation | undefined => {
  if (!isAPTAction(audit)) return undefined;

  const isUpdate = audit.request.capabilityName === 'install_os_updates';
  const parametersValid = hasExactEmptyParameters(audit);
  const reasons = getPolicyReasonCodes(audit);
  const summary = audit.result?.actionResultV2?.execution.summary?.trim() ?? '';
  const facts = isUpdate ? getUpdateFacts(summary) : getCleanupFacts(summary);
  const verification = audit.result?.actionResultV2?.verification;
  const recoveryRequired = getFactValue(facts, 'Recovery required') === 'Yes';
  const health = getFactValue(facts, 'Update system health');
  const rescanRequired = getFactValue(facts, 'Fresh rescan required') === 'Yes';

  let nextStep = 'Review the server policy and exact scope before making a decision.';
  if (audit.result?.actionResultV2 && facts.length === 0) {
    nextStep = 'Refresh this action record and review the canonical result before deciding what to do next.';
  } else if (verification?.status === 'confirmed') {
    nextStep = isUpdate
      ? 'Review the remaining-update and reboot facts. A reboot, if needed, is a separate governed action.'
      : 'Review the measured space reclaimed. Create another cleanup plan only if a fresh scan still shows pressure.';
  } else if (audit.result?.actionResultV2) {
    if (isUpdate && recoveryRequired) {
      nextStep = 'Do not retry. Repair the host update system, run a fresh scan, and create a new plan only after health is known.';
    } else if (isUpdate && health === 'Unknown') {
      nextStep = 'Do not retry automatically. Run a fresh host scan so update-system health and remaining updates are known.';
    } else if (!isUpdate && rescanRequired) {
      nextStep = 'Do not retry automatically. Run a fresh scan to measure the current cache pressure before creating another plan.';
    } else {
      nextStep = 'Review the separate execution and verification results, then rescan before creating another plan.';
    }
  }

  return {
    title: isUpdate ? 'Install operating system updates' : 'Clear downloaded package data',
    safetyPosture: reasons.includes('capability_auto_elevated')
      ? 'Elevated change'
      : reasons.includes('capability_auto_low_risk')
        ? 'Low-risk automation eligible'
        : 'Server policy controlled',
    approvalPosture:
      audit.plan.approvalPolicy === 'admin'
        ? 'Administrator approval required'
        : audit.plan.requiresApproval
          ? 'Explicit approval required'
          : 'No separate approval required by this plan',
    parameterAuthority: parametersValid
      ? 'None. This typed action accepts no command, path, package selection, removal choice, or reboot request.'
      : 'Unexpected parameters are present. Close this review and create a new plan.',
    parametersValid,
    authorityBoundary: isUpdate
      ? 'The agent may refresh update information and install the complete approved update set. It cannot remove packages or reboot the host.'
      : 'The agent may clear only downloaded package data on the pressured filesystem. It cannot choose paths, remove installed packages, or reboot the host.',
    facts,
    nextStep,
  };
};
