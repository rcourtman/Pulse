export function getInvestigationMessagesState(loading: boolean, hasMessages: boolean) {
  if (loading) {
    return {
      text: 'Loading messages...',
      empty: false,
    } as const;
  }

  if (!hasMessages) {
    return {
      text: 'No investigation messages available.',
      empty: true,
    } as const;
  }

  return {
    text: '',
    empty: false,
  } as const;
}

export function getRunHistoryEmptyState() {
  return {
    text: 'No patrol runs yet. Trigger a run to populate history.',
  } as const;
}

export function getInvestigationSectionState(loading: boolean, hasInvestigation: boolean) {
  if (loading) {
    return {
      text: 'Loading investigation...',
      empty: false,
    } as const;
  }

  if (!hasInvestigation) {
    return {
      text: 'No investigation data available. Enable patrol autonomy to investigate findings.',
      empty: true,
    } as const;
  }

  return {
    text: '',
    empty: false,
  } as const;
}
