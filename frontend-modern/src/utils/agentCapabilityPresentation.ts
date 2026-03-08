export type AgentCapability = 'agent' | 'docker' | 'kubernetes' | 'proxmox';

export function getAgentCapabilityLabel(capability: AgentCapability): string {
  switch (capability) {
    case 'agent':
      return 'Agent';
    case 'docker':
      return 'Docker';
    case 'kubernetes':
      return 'Kubernetes';
    case 'proxmox':
      return 'Proxmox';
  }
}

export function getAgentCapabilityBadgeClass(capability: AgentCapability): string {
  switch (capability) {
    case 'proxmox':
      return 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-300';
    case 'kubernetes':
      return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-300';
    default:
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300';
  }
}
