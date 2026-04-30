export interface AgentProfilesFeatureGateCopy {
  title: string;
  subtitle: string;
  body: string;
}

export function getAgentProfilesFeatureGateCopy(): AgentProfilesFeatureGateCopy {
  return {
    title: 'Agent Profiles',
    subtitle: 'Centralized agent configuration',
    body: 'Create reusable configuration profiles for your agents. Manage Docker monitoring, logging, and reporting intervals from a central location.',
  };
}

export function getAgentProfilesEmptyState(): string {
  return 'No profiles yet. Create one to get started.';
}

export function getAgentProfileAssignmentsEmptyState(): string {
  return 'No agents connected. Install an agent to assign profiles.';
}
