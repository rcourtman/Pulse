export function getRecoveryProtectedItemsEmptyState() {
  return {
    title: 'No protected items yet',
    description: 'Pulse hasn’t observed any protected items for this org yet.',
  } as const;
}

export function getRecoveryProtectedItemsLoadingState() {
  return {
    text: 'Loading protected items...',
  } as const;
}

export function getRecoveryProtectedItemsFailureState() {
  return {
    title: 'Failed to load protected items',
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
    description: 'Adjust your search, provider, method, status, or verification filters.',
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
