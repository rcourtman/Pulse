type FindingAlertIdentity = {
  alertIdentifier?: string;
};

export const getFindingAlertIdentifier = (
  finding: FindingAlertIdentity,
): string | undefined => {
  const canonical = typeof finding.alertIdentifier === 'string' ? finding.alertIdentifier.trim() : '';
  return canonical || undefined;
};

export const hasTriggeringAlert = (finding: FindingAlertIdentity): boolean =>
  getFindingAlertIdentifier(finding) !== undefined;
