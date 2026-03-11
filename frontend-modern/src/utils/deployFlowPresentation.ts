export function getDeployCandidatesLoadingState(): string {
  return 'Loading cluster nodes...';
}

export function getDeployNoSourceAgentsState(): string {
  return 'No online source agents found. At least one node in this cluster must have a connected Pulse agent to deploy to other nodes.';
}

export function getDeployNoCandidatesState(): string {
  return 'No nodes found in this cluster.';
}

export function getDeployInstallCommandLoadingState(): string {
  return 'Loading install command...';
}
