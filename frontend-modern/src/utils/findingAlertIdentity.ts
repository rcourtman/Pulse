type FindingAlertIdentity = {
  alertIdentifier?: string;
  alertId?: string;
};

export const getFindingAlertIdentifier = (
  finding: FindingAlertIdentity,
): string | undefined => {
  const canonical = typeof finding.alertIdentifier === 'string' ? finding.alertIdentifier.trim() : '';
  if (canonical) {
    return canonical;
  }
  const compatibility = typeof finding.alertId === 'string' ? finding.alertId.trim() : '';
  return compatibility || undefined;
};

export const hasTriggeringAlert = (finding: FindingAlertIdentity): boolean =>
  getFindingAlertIdentifier(finding) !== undefined;
