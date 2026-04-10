import { createSignal } from 'solid-js';
import type {
  SecurityStatus,
  SecurityStatusPresentationPolicy,
  SecurityStatusSessionCapabilities,
} from '@/types/config';
import { syncSessionCapabilities } from '@/stores/sessionCapabilities';

const DEFAULT_SESSION_PRESENTATION_POLICY: SecurityStatusPresentationPolicy = {
  demoMode: false,
  readOnly: false,
  hideCommercial: false,
  hideUpgrade: false,
};

const [sessionPresentationPolicy, setSessionPresentationPolicy] =
  createSignal<SecurityStatusPresentationPolicy>({
    ...DEFAULT_SESSION_PRESENTATION_POLICY,
  });
const [sessionPresentationPolicyResolved, setSessionPresentationPolicyResolved] = createSignal(false);

function normalizeSessionPresentationPolicy(
  policy?: Partial<SecurityStatusPresentationPolicy> | null,
  sessionCapabilities?: Partial<SecurityStatusSessionCapabilities> | null,
): SecurityStatusPresentationPolicy {
  const demoMode = policy?.demoMode === true || sessionCapabilities?.demoMode === true;
  return {
    ...DEFAULT_SESSION_PRESENTATION_POLICY,
    demoMode,
    readOnly: policy?.readOnly === true || demoMode,
    hideCommercial: policy?.hideCommercial === true || demoMode,
    hideUpgrade: policy?.hideUpgrade === true || demoMode,
  };
}

export function syncSessionPresentationPolicy(
  status?: Pick<SecurityStatus, 'sessionCapabilities' | 'presentationPolicy'> | null,
): SecurityStatusPresentationPolicy {
  syncSessionCapabilities(status);
  const next = normalizeSessionPresentationPolicy(
    status?.presentationPolicy,
    status?.sessionCapabilities,
  );
  setSessionPresentationPolicy(next);
  setSessionPresentationPolicyResolved(true);
  return next;
}

export function presentationPolicyIsDemoMode(): boolean {
  return sessionPresentationPolicy().demoMode;
}

export function presentationPolicyIsReadOnly(): boolean {
  return sessionPresentationPolicy().readOnly;
}

export function presentationPolicyHidesCommercialSurfaces(): boolean {
  return sessionPresentationPolicy().hideCommercial;
}

export function presentationPolicyHidesOrganizationSurfaces(): boolean {
  return sessionPresentationPolicy().demoMode;
}

export function presentationPolicyHidesUpgradePrompts(): boolean {
  return sessionPresentationPolicy().hideUpgrade;
}

export { sessionPresentationPolicy, sessionPresentationPolicyResolved };
