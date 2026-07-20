export type InfrastructureAddStep =
  | 'agent'
  | 'linux-host'
  | 'unraid'
  | 'docker'
  | 'kubernetes'
  | 'detect'
  | 'pve'
  | 'pbs'
  | 'pmg'
  | 'truenas'
  | 'vmware';
export type InfrastructurePanelStep = 'pick' | InfrastructureAddStep;

const INFRASTRUCTURE_BASE_PATH = '/settings/infrastructure';
export const INFRASTRUCTURE_ADD_QUERY_PARAM = 'add';
export const INFRASTRUCTURE_AGENT_DOCTOR_QUERY_PARAM = 'agentDoctor';
// Legacy deep links from platform update notices and bookmarks remain valid.
export const INFRASTRUCTURE_AGENT_UPDATES_QUERY_PARAM = 'agentUpdates';
export const INFRASTRUCTURE_AGENT_UPDATE_IDS_QUERY_PARAM = 'agents';

export function normalizeInfrastructurePanelStep(
  value: string | null | undefined,
): InfrastructurePanelStep | null {
  switch ((value || '').trim()) {
    case 'pick':
    case 'detect':
    case 'agent':
    case 'linux-host':
    case 'unraid':
    case 'docker':
    case 'kubernetes':
    case 'pve':
    case 'pbs':
    case 'pmg':
    case 'truenas':
    case 'vmware':
      return value!.trim() as InfrastructurePanelStep;
    default:
      return null;
  }
}

export function buildInfrastructureWorkspacePath(): string {
  return INFRASTRUCTURE_BASE_PATH;
}

const normalizeAgentUpdateConnectionID = (value: string | null | undefined): string | null => {
  const trimmed = (value || '').trim();
  if (!trimmed) return null;
  return trimmed.startsWith('agent:') ? trimmed : `agent:${trimmed}`;
};

export function buildInfrastructureAgentDoctorPath(
  agentIds: readonly (string | null | undefined)[] = [],
): string {
  const params = new URLSearchParams();
  params.set(INFRASTRUCTURE_AGENT_DOCTOR_QUERY_PARAM, '1');
  const normalizedAgentIds = Array.from(
    new Set(agentIds.map(normalizeAgentUpdateConnectionID).filter(Boolean) as string[]),
  ).sort((left, right) => left.localeCompare(right));
  for (const agentId of normalizedAgentIds) {
    params.append(INFRASTRUCTURE_AGENT_UPDATE_IDS_QUERY_PARAM, agentId);
  }
  return `${INFRASTRUCTURE_BASE_PATH}?${params.toString()}`;
}

/** @deprecated Use buildInfrastructureAgentDoctorPath. */
export const buildInfrastructureAgentUpdatesPath = buildInfrastructureAgentDoctorPath;

export function buildInfrastructureOnboardingPath(step: InfrastructurePanelStep = 'agent'): string {
  const params = new URLSearchParams();
  params.set(INFRASTRUCTURE_ADD_QUERY_PARAM, step);
  return `${INFRASTRUCTURE_BASE_PATH}?${params.toString()}`;
}

export function deriveAddStepFromSearch(search: string): InfrastructurePanelStep | null {
  const params = new URLSearchParams(search);
  return normalizeInfrastructurePanelStep(params.get(INFRASTRUCTURE_ADD_QUERY_PARAM));
}

export function deriveAddStepFromLocation(
  pathname: string,
  search: string,
): InfrastructurePanelStep | null {
  if (pathname !== INFRASTRUCTURE_BASE_PATH && pathname !== `${INFRASTRUCTURE_BASE_PATH}/`) {
    return null;
  }

  return deriveAddStepFromSearch(search);
}

export function deriveAgentDoctorFromLocation(pathname: string, search: string): boolean {
  if (pathname !== INFRASTRUCTURE_BASE_PATH && pathname !== `${INFRASTRUCTURE_BASE_PATH}/`) {
    return false;
  }

  const params = new URLSearchParams(search);
  return (
    params.get(INFRASTRUCTURE_AGENT_DOCTOR_QUERY_PARAM) === '1' ||
    params.get(INFRASTRUCTURE_AGENT_UPDATES_QUERY_PARAM) === '1'
  );
}

export function deriveAgentDoctorScopeFromLocation(pathname: string, search: string): string[] {
  if (!deriveAgentDoctorFromLocation(pathname, search)) {
    return [];
  }

  const params = new URLSearchParams(search);
  return Array.from(
    new Set(
      params
        .getAll(INFRASTRUCTURE_AGENT_UPDATE_IDS_QUERY_PARAM)
        .map(normalizeAgentUpdateConnectionID)
        .filter(Boolean) as string[],
    ),
  ).sort((left, right) => left.localeCompare(right));
}

/** @deprecated Use deriveAgentDoctorFromLocation. */
export const deriveAgentUpdatesFromLocation = deriveAgentDoctorFromLocation;

/** @deprecated Use deriveAgentDoctorScopeFromLocation. */
export const deriveAgentUpdateScopeFromLocation = deriveAgentDoctorScopeFromLocation;
