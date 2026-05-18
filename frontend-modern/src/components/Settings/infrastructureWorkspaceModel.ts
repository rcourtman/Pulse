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
  | 'availability'
  | 'truenas'
  | 'vmware';
export type InfrastructurePanelStep = 'pick' | InfrastructureAddStep;

const INFRASTRUCTURE_BASE_PATH = '/settings/infrastructure';
export const INFRASTRUCTURE_ADD_QUERY_PARAM = 'add';
const LEGACY_INSTALL_PATH = `${INFRASTRUCTURE_BASE_PATH}/install`;
const LEGACY_PLATFORMS_PATH = `${INFRASTRUCTURE_BASE_PATH}/platforms`;

function matchesExactPath(pathname: string, expected: string): boolean {
  return pathname === expected || pathname === `${expected}/`;
}

function matchesPathPrefix(pathname: string, expected: string): boolean {
  return pathname === expected || pathname.startsWith(`${expected}/`);
}

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
    case 'availability':
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

export function deriveAddStepFromLegacyPath(pathname: string): InfrastructurePanelStep | null {
  if (matchesPathPrefix(pathname, LEGACY_INSTALL_PATH)) return 'linux-host';
  if (matchesExactPath(pathname, LEGACY_PLATFORMS_PATH)) return 'pick';
  return null;
}

export function deriveAddStepFromSearch(search: string): InfrastructurePanelStep | null {
  const params = new URLSearchParams(search);
  return normalizeInfrastructurePanelStep(params.get(INFRASTRUCTURE_ADD_QUERY_PARAM));
}

export function deriveAddStepFromLocation(
  pathname: string,
  search: string,
): InfrastructurePanelStep | null {
  if (
    pathname === INFRASTRUCTURE_BASE_PATH ||
    pathname.startsWith(`${INFRASTRUCTURE_BASE_PATH}/`)
  ) {
    const stepFromQuery = deriveAddStepFromSearch(search);
    if (stepFromQuery) {
      return stepFromQuery;
    }
  }

  return deriveAddStepFromLegacyPath(pathname);
}
