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
