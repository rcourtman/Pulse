export function getRecoveryProtectedItemsEmptyState() {
  return {
    title: 'No protection coverage yet',
    description: 'Pulse has not observed recovery coverage for this org yet.',
  } as const;
}

export function getRecoveryProtectedItemsLoadingState() {
  return {
    text: 'Loading protection coverage...',
  } as const;
}

export function getRecoveryProtectedItemsFailureState() {
  return {
    title: 'Failed to load protection coverage',
  } as const;
}

export function getRecoveryActivityLoadingState() {
  return {
    text: 'Loading recovery activity...',
  } as const;
}

export function getRecoveryActivityEmptyState() {
  return {
    text: 'No recovery activity in the selected window.',
  } as const;
}

export function getRecoveryHistoryEmptyState() {
  return {
    title: 'No recovery history matches your filters',
    description: 'Adjust your search, platform, method, status, or verification filters.',
  } as const;
}

export function getRecoveryPointsLoadingState() {
  return {
    text: 'Loading recovery points...',
  } as const;
}

export function getRecoveryPointsFailureState() {
  return {
    title: 'Failed to load recovery points',
  } as const;
}
