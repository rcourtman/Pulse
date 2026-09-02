type FindingAlertIdentity = {
  alertIdentifier?: string;
};

export const getFindingAlertIdentifier = (finding: FindingAlertIdentity): string | undefined => {
  const canonical =
    typeof finding.alertIdentifier === 'string' ? finding.alertIdentifier.trim() : '';
  return canonical || undefined;
};

export const hasTriggeringAlert = (finding: FindingAlertIdentity): boolean =>
  getFindingAlertIdentifier(finding) !== undefined;

type FindingAlertMirror = {
  mirrorsAlertId?: string;
  status?: string;
};

/**
 * True when Patrol stamped this finding as restating an active alert (same
 * resource, same condition) and the finding is still active. Such findings are
 * demoted under the alert instead of listed as separate items; dismissed and
 * resolved findings keep their normal place because those views are about
 * Patrol's own history.
 */
export const isAlertMirroredFinding = (finding: FindingAlertMirror): boolean =>
  typeof finding.mirrorsAlertId === 'string' &&
  finding.mirrorsAlertId.trim() !== '' &&
  finding.status === 'active';
