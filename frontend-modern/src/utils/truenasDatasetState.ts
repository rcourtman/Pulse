// A read-only TrueNAS dataset raises no incident, so its state tag
// (datasetStateTag in internal/truenas/provider.go) is the only place the
// provider says why it is impaired. Locked and unmounted datasets raise
// incidents too, which callers prefer when present. Shares and apps carry
// their own state tags, so only dataset resources should be read this way.
const TRUENAS_DATASET_STATE_SUMMARIES: Record<string, string> = {
  'state:locked': 'Dataset is locked',
  'state:unmounted': 'Dataset is not mounted',
  'state:readonly': 'Dataset is read-only',
};

export const getTrueNASDatasetStateSummary = (
  tags: readonly string[] | undefined,
): string | null => {
  for (const tag of tags ?? []) {
    const summary = TRUENAS_DATASET_STATE_SUMMARIES[tag.trim().toLowerCase()];
    if (summary) return summary;
  }
  return null;
};
