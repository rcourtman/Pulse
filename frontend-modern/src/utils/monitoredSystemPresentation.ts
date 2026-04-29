const normalizeMonitoredSystemValue = (value: string | null | undefined): string =>
  value?.trim().toLowerCase() ?? '';

const titleCaseWords = (value: string): string =>
  value
    .split(/\s+/)
    .filter((part) => part.length > 0)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

const MONITORED_SYSTEM_LEDGER_PRESENTATION = {
  briefSummary:
    'Pulse counts top-level monitored systems. Child resources underneath them are included.',
  sectionTitle: 'Monitored Systems',
  panelTitle: 'Monitored System Ledger',
  disclosureButtonLabel: 'View counting rules',
  disclosureHideLabel: 'Hide counting rules',
  disclosureDefinition:
    'A monitored system is a top-level monitored root such as a Docker host, Kubernetes cluster, Proxmox node, standalone host, or TrueNAS system. Each root counts once no matter how Pulse collects it. Child resources like VMs, containers, pods, disks, backups, and services underneath that root are included.',
  ledgerDescription:
    'Review the top-level monitored systems Pulse has identified for reporting, migration continuity, and support context.',
  tableNameLabel: 'Name',
  tableStatusLabel: 'Status',
  tableLatestIncludedSignalLabel: 'Latest Included Signal',
  countedSystemBadgeLabel: 'Counts as 1 monitored system',
  groupedSourcesHeading: 'Grouped sources',
  countingExplanationHeading: 'Why this counts',
  continuityHeading: 'Legacy continuity',
  continuityPlanLimitLabel: 'Plan baseline',
  continuityEffectiveLimitLabel: 'Current baseline',
  continuityGrandfatheredFloorLabel: 'Observed legacy estate',
  continuityCaptureLabel: 'Verification',
  continuityCapturePendingLabel: 'Pending',
  continuityCaptureCapturedLabel: 'Captured',
  usageVerifyingLabel: 'Verifying…',
  remainingCapacityUnavailableLabel: 'Unavailable',
  unlimitedLimitLabel: 'Not metered',
  loadingState: {
    text: 'Loading monitored system usage…',
  },
  errorState: {
    title: 'Monitored system usage is temporarily unavailable.',
    retryingLabel: 'Trying again…',
    retryLabel: 'Try again',
  },
  unavailableState: {
    title: 'Verifying monitored-system inventory',
    fallbackMessage:
      'Pulse cannot currently verify monitored-system usage for this installation. Refresh after the monitoring runtime settles.',
    unsettledMessage:
      'Pulse is still collecting the first provider-owned inventory baseline. The monitored-system ledger will appear after that baseline completes.',
    rebuildPendingMessage:
      'Pulse has collected provider-owned inventory and is rebuilding the canonical monitored-system ledger. Usage will appear when that rebuild finishes.',
  },
  policyLoadingState: {
    title: 'Checking monitored-system visibility',
    message:
      'Pulse waits for the session visibility state before loading monitored-system usage details.',
  },
  hiddenState: {
    title: 'Monitored-system usage is hidden in demo mode',
    message:
      'The public demo uses sample infrastructure data, so Pulse hides counted-system totals and billing actions instead of creating a demo license.',
  },
  countingDetailsCollapsedLabel: 'View counting details',
  countingDetailsExpandedLabel: 'Hide counting details',
  currentStatusHeading: 'Current status',
  latestIncludedSignalSummaryLabel: 'Latest included signal',
  includedCollectionPathsHeading: 'Included collection paths',
  emptyState: 'No monitored systems counted.',
  noIncludedSignalLabel: 'No included signal yet.',
  fallbackExplanationSummary:
    'Pulse counts this top-level collection path as one monitored system.',
  statusSummaryByStatus: {
    online: 'All included top-level collection paths currently report online status.',
    warning:
      'At least one included top-level collection path is degraded, so Pulse marks this monitored system as warning.',
    offline:
      'At least one included source is offline or disconnected, so Pulse marks this monitored system as offline.',
    unknown: 'Pulse cannot determine a canonical runtime status for this monitored system yet.',
  },
  limitBanner: {
    reviewPolicyLabel: 'Review continuity',
    installCollectorsLabel: 'Install v6 collectors',
    overflowSummaryPrefix: 'A temporary setup slot is active',
    legacyConnectionSuffix:
      'that are folded into the canonical monitored-system ledger when the same top-level system is discovered canonically.',
  },
  admissionPreview: {
    requiredTitle: 'Preview monitored-system impact before saving',
    requiredMessage:
      'Pulse must preview the monitored-system impact for this platform connection before it can be saved.',
    fallbackTitle: 'Monitored-system impact',
    exceedsPolicyTitle: 'This change needs continuity review before saving',
    addsSystemsTitle: 'This change adds monitored systems',
    removesSystemsTitle: 'This change removes monitored systems',
    unchangedTitle: 'This change keeps monitored-system count unchanged',
    unavailableTitle: 'Monitored-system verification is temporarily unavailable',
    unavailableFallbackMessage:
      'Pulse cannot verify monitored-system impact right now, so this connection cannot be saved yet. Retry preview in a moment.',
    unavailableUnsettledMessage:
      'Pulse is still settling provider-owned inventory for this platform connection, so the monitored-system check is not safe yet. Retry preview after the first baseline finishes.',
    unavailableRebuildPendingMessage:
      'Pulse has settled provider-owned inventory and is rebuilding the canonical monitored-system view, so this connection cannot be saved yet. Retry preview in a moment.',
    saveBlockedLimitMessage: 'This change needs monitored-system review before saving',
    saveBlockedLoadingMessage: 'Wait for the monitored-system impact preview to finish',
  },
} as const;

const MONITORED_SYSTEM_USAGE_UNAVAILABLE_ERROR_CODE = 'monitored_system_usage_unavailable';

export type MonitoredSystemLegacyConnectionCounts = {
  proxmox_nodes: number;
  docker_hosts: number;
  kubernetes_clusters: number;
};

export type MonitoredSystemLimitUsageStatus = {
  current?: number | null;
  limit?: number | null;
  current_available?: boolean | null;
  current_unavailable_reason?: string | null;
  state?: string | null;
};

export type MonitoredSystemCapacityStatus = {
  mode?: string | null;
  urgency?: string | null;
  current?: number | null;
  limit?: number | null;
  current_available?: boolean | null;
  current_unavailable_reason?: string | null;
  available_slots?: number | null;
  overage?: number | null;
  reason?: string | null;
  blocks_new_systems?: boolean | null;
  existing_monitoring_continues?: boolean | null;
};

type ResolvedMonitoredSystemCapacityStatus = {
  mode: string;
  urgency: string;
  current: number;
  limit: number;
  current_available: boolean;
  current_unavailable_reason?: string;
  available_slots: number;
  overage: number;
  reason?: string;
  blocks_new_systems: boolean;
  existing_monitoring_continues: boolean;
};

export type MonitoredSystemAdmissionPreviewUnavailableState = {
  reason: string | null;
  title: string;
  message: string;
};

export type MonitoredSystemCapacitySectionModel = {
  stats: Array<{ label: string; value: string }>;
  statusMessage: string;
  detailMessage?: string;
  explanation?: {
    label: string;
    body: string;
  };
};

export type MonitoredSystemAdmissionPreviewSaveState = {
  preview?: { would_exceed_limit?: boolean | null } | null;
  unavailableState?: MonitoredSystemAdmissionPreviewUnavailableState | null;
  error?: string | null;
  loading?: boolean | null;
};

export type MonitoredSystemAdmissionPreviewTitleInput = {
  current_count?: number | null;
  projected_count?: number | null;
  would_exceed_limit?: boolean | null;
};

export type MonitoredSystemAdmissionPreviewSummaryInput =
  MonitoredSystemAdmissionPreviewTitleInput & {
    limit?: number | null;
  };

const normalizeAdmissionPreviewCount = (count: number | null | undefined): number =>
  typeof count === 'number' && Number.isFinite(count) ? Math.max(0, count) : 0;

const formatAdmissionPreviewCount = (count: number): string =>
  `${count} monitored ${count === 1 ? 'system' : 'systems'}`;

const formatAdmissionPreviewDelta = (delta: number): string =>
  delta > 0 ? `+${delta}` : `${delta}`;

export function getMonitoredSystemLedgerPresentation() {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION;
}

export function getMonitoredSystemBriefSummary(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.briefSummary;
}

export function getMonitoredSystemDisclosureToggleLabel(open: boolean): string {
  return open
    ? MONITORED_SYSTEM_LEDGER_PRESENTATION.disclosureHideLabel
    : MONITORED_SYSTEM_LEDGER_PRESENTATION.disclosureButtonLabel;
}

export function getMonitoredSystemDisclosureDefinition(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.disclosureDefinition;
}

export function getMonitoredSystemLedgerDescription(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.ledgerDescription;
}

export function getMonitoredSystemLedgerLoadingState() {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.loadingState;
}

export function getMonitoredSystemLedgerErrorState() {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.errorState;
}

export function getMonitoredSystemLedgerUnavailableState() {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.unavailableState;
}

export function getMonitoredSystemLedgerPolicyLoadingState() {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.policyLoadingState;
}

export function getMonitoredSystemLedgerHiddenState() {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.hiddenState;
}

export function formatMonitoredSystemUsageUnavailableMessage(reason?: string | null): string {
  switch (normalizeMonitoredSystemValue(reason)) {
    case 'supplemental_inventory_unsettled':
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.unavailableState.unsettledMessage;
    case 'supplemental_inventory_rebuild_pending':
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.unavailableState.rebuildPendingMessage;
    default:
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.unavailableState.fallbackMessage;
  }
}

export function formatMonitoredSystemLedgerUnavailableMessage(reason?: string | null): string {
  return formatMonitoredSystemUsageUnavailableMessage(reason);
}

export function isMonitoredSystemLimitUsageAvailable(
  limit?: MonitoredSystemLimitUsageStatus | null,
): boolean {
  return limit?.current_available !== false;
}

export function getMonitoredSystemLimitUnavailableReason(
  limit?: MonitoredSystemLimitUsageStatus | null,
  capacity?: MonitoredSystemCapacityStatus | null,
): string | undefined {
  const resolved = resolveMonitoredSystemCapacityStatus(capacity, limit);
  if (resolved?.current_available) return undefined;
  return resolved?.current_unavailable_reason?.trim() || undefined;
}

function deriveMonitoredSystemLimitUrgency(current: number, limit: number): string {
  if (limit <= 0) return 'ok';
  if (current >= limit) return 'enforced';
  if (limit > 1 && limit <= 10) {
    return current >= limit - 1 ? 'warning' : 'ok';
  }
  return current * 10 >= limit * 9 ? 'warning' : 'ok';
}

function deriveMonitoredSystemCapacityStatus(
  limit?: MonitoredSystemLimitUsageStatus | null,
): ResolvedMonitoredSystemCapacityStatus | undefined {
  if (!limit) return undefined;

  const currentAvailable = isMonitoredSystemLimitUsageAvailable(limit);
  const current = typeof limit.current === 'number' ? limit.current : 0;
  const planLimit = typeof limit.limit === 'number' ? limit.limit : 0;
  const urgency =
    normalizeMonitoredSystemValue(limit.state ?? undefined) ||
    deriveMonitoredSystemLimitUrgency(current, planLimit);

  if (!currentAvailable) {
    return {
      mode: 'usage_unavailable',
      urgency: 'ok',
      current: 0,
      limit: planLimit,
      current_available: false,
      current_unavailable_reason: limit.current_unavailable_reason?.trim() || undefined,
      available_slots: 0,
      overage: 0,
      reason: undefined,
      blocks_new_systems: false,
      existing_monitoring_continues: false,
    };
  }

  if (planLimit <= 0) {
    return {
      mode: 'unlimited',
      urgency: 'ok',
      current,
      limit: 0,
      current_available: true,
      available_slots: 0,
      overage: 0,
      reason: undefined,
      blocks_new_systems: false,
      existing_monitoring_continues: true,
    };
  }

  if (current > planLimit) {
    return {
      mode: 'over_limit_frozen',
      urgency: 'enforced',
      current,
      limit: planLimit,
      current_available: true,
      available_slots: 0,
      overage: current - planLimit,
      reason: 'preexisting_usage',
      blocks_new_systems: true,
      existing_monitoring_continues: true,
    };
  }

  if (current === planLimit) {
    return {
      mode: 'at_limit_blocking_new',
      urgency: 'enforced',
      current,
      limit: planLimit,
      current_available: true,
      available_slots: 0,
      overage: 0,
      reason: 'limit_reached',
      blocks_new_systems: true,
      existing_monitoring_continues: true,
    };
  }

  return {
    mode: 'within_limit',
    urgency,
    current,
    limit: planLimit,
    current_available: true,
    available_slots: planLimit - current,
    overage: 0,
    reason: undefined,
    blocks_new_systems: false,
    existing_monitoring_continues: true,
  };
}

export function resolveMonitoredSystemCapacityStatus(
  capacity?: MonitoredSystemCapacityStatus | null,
  limit?: MonitoredSystemLimitUsageStatus | null,
): ResolvedMonitoredSystemCapacityStatus | undefined {
  const fallback = deriveMonitoredSystemCapacityStatus(limit);
  if (!capacity) {
    return fallback;
  }

  const current =
    typeof capacity.current === 'number' ? capacity.current : (fallback?.current ?? 0);
  const planLimit = typeof capacity.limit === 'number' ? capacity.limit : (fallback?.limit ?? 0);
  const currentAvailable =
    typeof capacity.current_available === 'boolean'
      ? capacity.current_available
      : (fallback?.current_available ?? true);
  const mode =
    normalizeMonitoredSystemValue(capacity.mode ?? undefined) ||
    fallback?.mode ||
    'usage_unavailable';
  const urgency =
    normalizeMonitoredSystemValue(capacity.urgency ?? undefined) ||
    fallback?.urgency ||
    deriveMonitoredSystemLimitUrgency(current, planLimit);
  const reason =
    normalizeMonitoredSystemValue(capacity.reason ?? undefined) || fallback?.reason || undefined;

  return {
    mode,
    urgency,
    current,
    limit: planLimit,
    current_available: currentAvailable,
    current_unavailable_reason:
      capacity.current_unavailable_reason?.trim() ||
      fallback?.current_unavailable_reason ||
      undefined,
    available_slots:
      typeof capacity.available_slots === 'number'
        ? capacity.available_slots
        : (fallback?.available_slots ?? Math.max(planLimit - current, 0)),
    overage:
      typeof capacity.overage === 'number'
        ? capacity.overage
        : (fallback?.overage ?? Math.max(current - planLimit, 0)),
    reason,
    blocks_new_systems:
      typeof capacity.blocks_new_systems === 'boolean'
        ? capacity.blocks_new_systems
        : (fallback?.blocks_new_systems ?? false),
    existing_monitoring_continues:
      typeof capacity.existing_monitoring_continues === 'boolean'
        ? capacity.existing_monitoring_continues
        : (fallback?.existing_monitoring_continues ?? currentAvailable),
  };
}

function formatMonitoredSystemCount(value: number): string {
  return `${value} monitored system${value === 1 ? '' : 's'}`;
}

export function getMonitoredSystemLimitUsageSummary(
  limit?: MonitoredSystemLimitUsageStatus | null,
  capacity?: MonitoredSystemCapacityStatus | null,
): string {
  const resolved = resolveMonitoredSystemCapacityStatus(capacity, limit);
  if (!resolved || !resolved.current_available) {
    return MONITORED_SYSTEM_LEDGER_PRESENTATION.usageVerifyingLabel;
  }
  return formatMonitoredSystemCount(resolved.current);
}

export function getMonitoredSystemLimitCapacityStatusSummary(
  limit?: MonitoredSystemLimitUsageStatus | null,
  capacity?: MonitoredSystemCapacityStatus | null,
): string {
  const resolved = resolveMonitoredSystemCapacityStatus(capacity, limit);
  if (!resolved || !resolved.current_available) {
    return MONITORED_SYSTEM_LEDGER_PRESENTATION.remainingCapacityUnavailableLabel;
  }

  switch (resolved.mode) {
    case 'unlimited':
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.unlimitedLimitLabel;
    case 'over_limit_frozen':
      return 'Continuity review';
    case 'at_limit_blocking_new':
      return 'Continuity review';
    default:
      return 'Healthy';
  }
}

export function getMonitoredSystemLimitContextSummary(
  limit?: MonitoredSystemLimitUsageStatus | null,
  capacity?: MonitoredSystemCapacityStatus | null,
): string {
  const resolved = resolveMonitoredSystemCapacityStatus(capacity, limit);
  if (!resolved) {
    return '';
  }
  if (!resolved.current_available) {
    return formatMonitoredSystemUsageUnavailableMessage(resolved.current_unavailable_reason);
  }

  switch (resolved.mode) {
    case 'unlimited':
      return 'Self-hosted monitoring is included for this instance.';
    case 'over_limit_frozen':
      if (resolved.reason === 'legacy_migration_capture_pending') {
        return 'Pulse is verifying legacy v5 continuity for this installation. Existing monitoring remains visible while new top-level additions wait for verification to finish.';
      }
      return 'Existing monitoring remains visible. New top-level additions are paused until this legacy continuity state is reviewed.';
    case 'at_limit_blocking_new':
      return 'Existing monitoring remains visible. New top-level additions are paused until this legacy continuity state is reviewed.';
    default:
      return '';
  }
}

export function buildMonitoredSystemCapacitySectionModel(
  limit?: MonitoredSystemLimitUsageStatus | null,
  capacity?: MonitoredSystemCapacityStatus | null,
): MonitoredSystemCapacitySectionModel | null {
  const resolved = resolveMonitoredSystemCapacityStatus(capacity, limit);
  if (!resolved) {
    return null;
  }
  if (resolved.limit <= 0) {
    return null;
  }
  if (
    resolved.current_available &&
    resolved.mode !== 'over_limit_frozen' &&
    resolved.mode !== 'at_limit_blocking_new'
  ) {
    return null;
  }

  const includedValue =
    resolved.limit > 0
      ? String(resolved.limit)
      : MONITORED_SYSTEM_LEDGER_PRESENTATION.remainingCapacityUnavailableLabel;

  const stats = [
    {
      label: 'Monitored',
      value: getMonitoredSystemLimitUsageSummary(limit, capacity),
    },
    {
      label: 'Baseline',
      value: includedValue,
    },
    {
      label: 'Status',
      value: getMonitoredSystemLimitCapacityStatusSummary(limit, capacity),
    },
  ];

  if (!resolved.current_available) {
    return {
      stats,
      statusMessage: 'Pulse is verifying monitored-system usage for this installation.',
      detailMessage: formatMonitoredSystemUsageUnavailableMessage(
        resolved.current_unavailable_reason,
      ),
    };
  }

  switch (resolved.mode) {
    case 'unlimited':
      return {
        stats,
        statusMessage: 'Self-hosted monitoring is included for this instance.',
      };
    case 'at_limit_blocking_new':
      return {
        stats,
        statusMessage: 'Existing monitoring remains visible. New top-level additions need review.',
        detailMessage:
          'Review the legacy continuity state before adding another top-level monitored system.',
      };
    case 'over_limit_frozen':
      if (resolved.reason === 'legacy_migration_capture_pending') {
        return {
          stats,
          statusMessage:
            'Existing monitoring remains visible. New top-level additions wait for verification.',
          detailMessage: 'Pulse is still verifying legacy v5 continuity for this installation.',
          explanation: {
            label: 'Why is continuity still pending?',
            body: `Pulse is still verifying legacy v5 continuity for this installation. It has already identified ${resolved.current} monitored systems for continuity reporting, and existing monitoring remains visible while new top-level additions wait for verification to finish.`,
          },
        };
      }
      return {
        stats,
        statusMessage: 'Existing monitoring remains visible. New top-level additions need review.',
        detailMessage:
          'Review the legacy continuity state before adding another top-level monitored system.',
        explanation: {
          label: 'Why does this need review?',
          body: `Pulse has already identified ${resolved.current} monitored systems for this installation. Existing monitoring remains visible, but new top-level additions are paused until this legacy continuity state is reviewed.`,
        },
      };
    default:
      return {
        stats,
        statusMessage: 'Current monitored-system accounting is healthy.',
      };
  }
}

export function isMonitoredSystemLimitUrgent(
  limit?: MonitoredSystemLimitUsageStatus | null,
  capacity?: MonitoredSystemCapacityStatus | null,
): boolean {
  const resolved = resolveMonitoredSystemCapacityStatus(capacity, limit);
  if (!resolved?.current_available) return false;
  const state = normalizeMonitoredSystemValue(resolved.urgency);
  return state === 'warning' || state === 'enforced';
}

export function getMonitoredSystemCountingDetailsToggleLabel(expanded: boolean): string {
  return expanded
    ? MONITORED_SYSTEM_LEDGER_PRESENTATION.countingDetailsExpandedLabel
    : MONITORED_SYSTEM_LEDGER_PRESENTATION.countingDetailsCollapsedLabel;
}

export function getMonitoredSystemExplanationFallbackSummary(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.fallbackExplanationSummary;
}

export function getMonitoredSystemStatusFallbackSummary(
  status: 'online' | 'warning' | 'offline' | 'unknown' = 'unknown',
): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.statusSummaryByStatus[status];
}

export function getMonitoredSystemLimitReviewPolicyLabel(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.limitBanner.reviewPolicyLabel;
}

export function getMonitoredSystemLimitInstallCollectorsLabel(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.limitBanner.installCollectorsLabel;
}

export function getMonitoredSystemAdmissionPreviewUnavailableTitle(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.unavailableTitle;
}

export function getMonitoredSystemAdmissionPreviewRequiredState(): {
  title: string;
  message: string;
} {
  return {
    title: MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.requiredTitle,
    message: MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.requiredMessage,
  };
}

export function getMonitoredSystemAdmissionPreviewTitle(
  preview: MonitoredSystemAdmissionPreviewTitleInput | null | undefined,
): string {
  const presentation = MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview;
  if (!preview) return presentation.fallbackTitle;
  if (preview.would_exceed_limit) return presentation.exceedsPolicyTitle;

  const current = normalizeAdmissionPreviewCount(preview.current_count);
  const projected =
    typeof preview.projected_count === 'number' && Number.isFinite(preview.projected_count)
      ? normalizeAdmissionPreviewCount(preview.projected_count)
      : current;
  const delta = projected - current;

  if (delta > 0) return presentation.addsSystemsTitle;
  if (delta < 0) return presentation.removesSystemsTitle;
  return presentation.unchangedTitle;
}

export function formatMonitoredSystemAdmissionPreviewSummary(
  preview: MonitoredSystemAdmissionPreviewSummaryInput,
): string {
  const current = normalizeAdmissionPreviewCount(preview.current_count);
  const projected =
    typeof preview.projected_count === 'number' && Number.isFinite(preview.projected_count)
      ? normalizeAdmissionPreviewCount(preview.projected_count)
      : current;
  const limit = normalizeAdmissionPreviewCount(preview.limit);
  const delta = projected - current;
  const policySuffix =
    preview.would_exceed_limit && limit > 0
      ? `, above the current verified baseline of ${formatAdmissionPreviewCount(limit)}`
      : '';
  const currentSummary = `Pulse currently counts ${formatAdmissionPreviewCount(current)}.`;

  if (delta !== 0) {
    return `${currentSummary} Saving this change would bring the count to ${formatAdmissionPreviewCount(
      projected,
    )} (${formatAdmissionPreviewDelta(delta)})${policySuffix}.`;
  }

  return `${currentSummary} Saving this change would keep the count at ${formatAdmissionPreviewCount(
    projected,
  )}${policySuffix}.`;
}

export function formatMonitoredSystemAdmissionPreviewUnavailableMessage(reason?: string): string {
  switch (normalizeMonitoredSystemValue(reason)) {
    case 'supplemental_inventory_unsettled':
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.unavailableUnsettledMessage;
    case 'supplemental_inventory_rebuild_pending':
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.unavailableRebuildPendingMessage;
    default:
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.unavailableFallbackMessage;
  }
}

export function buildMonitoredSystemAdmissionPreviewUnavailableState(input: {
  code?: string | null;
  reason?: string | null;
}): MonitoredSystemAdmissionPreviewUnavailableState | null {
  if (normalizeMonitoredSystemValue(input.code) !== MONITORED_SYSTEM_USAGE_UNAVAILABLE_ERROR_CODE) {
    return null;
  }

  const reason = input.reason?.trim() || null;
  return {
    reason,
    title: getMonitoredSystemAdmissionPreviewUnavailableTitle(),
    message: formatMonitoredSystemAdmissionPreviewUnavailableMessage(reason ?? undefined),
  };
}

export function isMonitoredSystemAdmissionPreviewResolvedSafely(
  state: MonitoredSystemAdmissionPreviewSaveState,
): boolean {
  return (
    !state.loading &&
    Boolean(state.preview) &&
    state.preview?.would_exceed_limit !== true &&
    !state.unavailableState &&
    !state.error?.trim()
  );
}

export function getMonitoredSystemAdmissionPreviewSaveBlockedMessage(
  state: MonitoredSystemAdmissionPreviewSaveState,
): string | null {
  if (isMonitoredSystemAdmissionPreviewResolvedSafely(state)) {
    return null;
  }

  if (state.loading) {
    return MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.saveBlockedLoadingMessage;
  }
  if (state.unavailableState) {
    return state.unavailableState.message;
  }
  if (state.preview?.would_exceed_limit) {
    return MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.saveBlockedLimitMessage;
  }

  const error = state.error?.trim();
  if (error) {
    return error;
  }

  return MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.requiredMessage;
}

export function formatMonitoredSystemLimitSummary(
  limit: {
    current: number;
    limit: number;
    current_available?: boolean | null;
    current_unavailable_reason?: string | null;
    state?: string | null;
  },
  capacity?: MonitoredSystemCapacityStatus | null,
): string {
  const resolved = resolveMonitoredSystemCapacityStatus(capacity, limit);
  if (!resolved || !resolved.current_available) {
    return MONITORED_SYSTEM_LEDGER_PRESENTATION.usageVerifyingLabel;
  }

  switch (resolved.mode) {
    case 'over_limit_frozen':
      if (resolved.reason === 'legacy_migration_capture_pending') {
        return `Continuity verification pending. ${formatMonitoredSystemCount(
          resolved.current,
        )}.`;
      }
      return `Continuity review needed. ${formatMonitoredSystemCount(resolved.current)}.`;
    case 'at_limit_blocking_new':
      return `Continuity review needed. ${formatMonitoredSystemCount(resolved.current)}.`;
    case 'unlimited':
      return `${formatMonitoredSystemCount(resolved.current)}.`;
    default:
      return `${formatMonitoredSystemCount(resolved.current)}.`;
  }
}

export function formatMonitoredSystemLegacyConnectionBreakdown(
  counts: MonitoredSystemLegacyConnectionCounts,
): string {
  const parts: string[] = [];

  if (counts.proxmox_nodes > 0) {
    parts.push(`${counts.proxmox_nodes} Proxmox ${counts.proxmox_nodes === 1 ? 'node' : 'nodes'}`);
  }
  if (counts.docker_hosts > 0) {
    parts.push(`${counts.docker_hosts} Docker ${counts.docker_hosts === 1 ? 'host' : 'hosts'}`);
  }
  if (counts.kubernetes_clusters > 0) {
    parts.push(
      `${counts.kubernetes_clusters} Kubernetes ${
        counts.kubernetes_clusters === 1 ? 'cluster' : 'clusters'
      }`,
    );
  }

  return parts.join(', ');
}

export function formatMonitoredSystemMigrationMessage(
  counts: MonitoredSystemLegacyConnectionCounts,
): string {
  const total = counts.proxmox_nodes + counts.docker_hosts + counts.kubernetes_clusters;
  if (total <= 0) return '';

  const noun = total === 1 ? 'resource' : 'resources';
  const breakdown = formatMonitoredSystemLegacyConnectionBreakdown(counts);
  return `You also have ${total} ${noun} connected via API or legacy collectors${
    breakdown ? ` (${breakdown})` : ''
  } ${MONITORED_SYSTEM_LEDGER_PRESENTATION.limitBanner.legacyConnectionSuffix}`;
}

export function formatMonitoredSystemOverflowSummary(daysRemaining: number | undefined): string {
  if (!daysRemaining) return '';
  return `${MONITORED_SYSTEM_LEDGER_PRESENTATION.limitBanner.overflowSummaryPrefix} (${daysRemaining}d remaining)`;
}

export function formatMonitoredSystemLatestIncludedSignalSentence(signal: {
  attribution: string;
  relative: string;
}): string {
  return `${MONITORED_SYSTEM_LEDGER_PRESENTATION.latestIncludedSignalSummaryLabel}: ${signal.attribution}, reported ${signal.relative}.`;
}

export function formatMonitoredSystemGroupedSourcesLabel(count: number): string {
  return `${count} grouped ${count === 1 ? 'source' : 'sources'}`;
}

export function getMonitoredSystemSourceLabel(source: string | undefined): string {
  switch (normalizeMonitoredSystemValue(source)) {
    case 'agent':
      return 'Agent';
    case 'docker':
      return 'Docker';
    case 'kubernetes':
      return 'Kubernetes';
    case 'multiple':
      return 'Multiple Sources';
    case 'pbs':
      return 'PBS';
    case 'pmg':
      return 'PMG';
    case 'proxmox':
      return 'Proxmox';
    case 'vmware':
      return 'VMware';
    case 'truenas':
      return 'TrueNAS';
    case '':
    case 'unknown':
      return '';
    default:
      return source?.trim() ?? '';
  }
}

export function getMonitoredSystemSurfaceTypeLabel(type: string | undefined): string {
  switch (normalizeMonitoredSystemValue(type)) {
    case 'agent':
      return 'Host';
    case 'docker-host':
      return 'Docker Host';
    case 'host':
      return 'Host';
    case 'kubernetes-cluster':
      return 'Kubernetes Cluster';
    case 'pbs-server':
      return 'PBS Server';
    case 'pmg-server':
      return 'PMG Server';
    case 'proxmox-node':
      return 'Proxmox Node';
    case 'truenas-system':
      return 'TrueNAS System';
    case '':
      return 'System';
    default:
      return titleCaseWords((type ?? '').trim().replace(/[-_]+/g, ' '));
  }
}

export function formatMonitoredSystemSurfaceAttribution(surface: {
  name: string;
  type?: string;
  source?: string;
}): string {
  const name = surface.name?.trim() || 'Unnamed source';
  const typeLabel = getMonitoredSystemSurfaceTypeLabel(surface.type);
  const sourceLabel = getMonitoredSystemSourceLabel(surface.source);
  if (sourceLabel === '' || sourceLabel.toLowerCase() === typeLabel.toLowerCase()) {
    return `${name} (${typeLabel})`;
  }
  return `${name} (${typeLabel} via ${sourceLabel})`;
}
