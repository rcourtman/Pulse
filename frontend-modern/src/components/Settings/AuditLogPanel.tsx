import { Show, createSignal } from 'solid-js';
import Shield from 'lucide-solid/icons/shield';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import Play from 'lucide-solid/icons/play';
import Download from 'lucide-solid/icons/download';
import ChevronDown from 'lucide-solid/icons/chevron-down';
import ChevronRight from 'lucide-solid/icons/chevron-right';
import ShieldAlert from 'lucide-solid/icons/shield-alert';
import { Button } from '@/components/shared/Button';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { EmptyState } from '@/components/shared/EmptyState';
import { showTooltip, hideTooltip } from '@/components/shared/Tooltip';
import { HelpIcon } from '@/components/shared/HelpIcon';
import { UpgradeButtonLink } from '@/components/shared/UpgradeLink';
import { FilterBar, type FilterDef, type FilterSelectOption } from '@/components/shared/FilterBar';
import { FormSelect } from '@/components/shared/FormSelect';
import { SearchField } from '@/components/shared/SearchField';
import Toggle from '@/components/shared/Toggle';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import {
  AUDIT_REFRESH_BUTTON_CLASS,
  AUDIT_VERIFY_ALL_BUTTON_CLASS,
  AUDIT_VERIFY_ROW_BUTTON_CLASS,
  AUDIT_TOOLBAR_BUTTON_CLASS,
  AUDIT_EVENT_CONFIG_CHANGE_LABEL,
  AUDIT_EVENT_FILTER_ALL_LABEL,
  AUDIT_SUCCESS_FILTER_FAILED_ONLY_LABEL,
  AUDIT_SUCCESS_FILTER_SUCCESS_ONLY_LABEL,
  AUDIT_VERIFICATION_FILTER_ALL_LABEL,
  AUDIT_VERIFICATION_FILTER_NEEDS_LABEL,
  getAuditEventStatusPresentation,
  getAuditEventTypeBadgeClass,
  getAuditLogFeatureGateCopy,
  getAuditLogEmptyState,
  getAuditLogLoadingState,
  getAuditVerificationBadgePresentation,
} from '@/utils/auditLogPresentation';
import {
  useAuditLogPanelState,
  type AuditLogPanelProps,
} from '@/components/Settings/useAuditLogPanelState';
import { InlineNotice } from '@/components/shared/InlineNotice';

const AUDIT_PAGE_BUTTON_CLASS =
  'min-h-11 sm:min-h-10 px-1 sm:px-3 py-2.5 text-sm font-medium text-base-content bg-surface border border-border rounded-md hover:bg-surface-hover disabled:opacity-50';

const auditEventOutcomeLabel = (success: boolean) => (success ? 'Success' : 'Failed');

export default function AuditLogPanel(props: AuditLogPanelProps) {
  const { isMobile } = useBreakpoint();
  const [expandedEvents, setExpandedEvents] = createSignal<Record<string, boolean>>({});
  const toggleExpandedEvent = (eventId: string) => {
    setExpandedEvents((prev) => ({ ...prev, [eventId]: !prev[eventId] }));
  };
  const {
    accessAllowed,
    activeFilterCount,
    auditLoggingEnabled,
    autoVerifyEnabled,
    autoVerifyLimit,
    cancelVerification,
    clearFilters,
    dateRangeFilter,
    error,
    eventFilter,
    exportCsv,
    exportJson,
    filteredEvents,
    goToFirstPage,
    goToLastPage,
    goToNextPage,
    goToPreviousPage,
    hasLogFilters,
    hasNextPage,
    hasResumeEvents,
    hasSignedEvents,
    isPersistent,
    loading,
    pageInput,
    pageNumber,
    pageRangeText,
    pageSize,
    refresh,
    resumeCount,
    resumeVerification,
    resetPreferences,
    searchQuery,
    setAutoVerifyEnabled,
    setAutoVerifyLimit,
    setDateRangeFilter,
    setEventFilter,
    setPageInput,
    setPageSize,
    setSearchQuery,
    setSuccessFilter,
    setUserFilter,
    setVerificationFilter,
    showLogFilters,
    showUpgradePaywall,
    showFeatureGateAction,
    submitPageInput,
    successFilter,
    totalEvents,
    totalPages,
    upgradeDestination,
    upgradeActionLabel,
    paidRuntimeRequired,
    verification,
    verificationFilter,
    verificationSummary,
    verificationNotice,
    verifyAll,
    verifyAllLabel,
    verifyCanceled,
    verifyEvent,
    verifying,
    verifyingAll,
    userFilter,
  } = useAuditLogPanelState(props);

  const formatTimestamp = (ts: string) => {
    const date = new Date(ts);
    return date.toLocaleString();
  };
  const featureGateCopy = () =>
    getAuditLogFeatureGateCopy({ paidRuntimeRequired: paidRuntimeRequired() });
  const hasPageVerificationFilter = () => verificationFilter() !== 'all';
  const loadedEventCount = () => verificationSummary().total;
  const logEmptyState = () => getAuditLogEmptyState(hasLogFilters());

  const auditEventOptions = (): FilterSelectOption[] => [
    { value: '', label: AUDIT_EVENT_FILTER_ALL_LABEL },
    { value: 'login', label: 'Login' },
    { value: 'logout', label: 'Logout' },
    { value: 'config_change', label: AUDIT_EVENT_CONFIG_CHANGE_LABEL },
    { value: 'startup', label: 'Startup' },
  ];
  const auditSuccessOptions = (): FilterSelectOption[] => [
    { value: 'all', label: 'All' },
    { value: 'success', label: AUDIT_SUCCESS_FILTER_SUCCESS_ONLY_LABEL },
    { value: 'failed', label: AUDIT_SUCCESS_FILTER_FAILED_ONLY_LABEL },
  ];
  const auditVerificationOptions = (): FilterSelectOption[] => [
    { value: 'all', label: AUDIT_VERIFICATION_FILTER_ALL_LABEL },
    { value: 'needs', label: AUDIT_VERIFICATION_FILTER_NEEDS_LABEL },
    { value: 'verified', label: 'Verified' },
    { value: 'failed', label: 'Check failed or error' },
  ];
  const auditDateRangeOptions = (): FilterSelectOption[] => [
    { value: 'all', label: 'All time' },
    { value: '24h', label: 'Last 24 hours' },
    { value: '7d', label: 'Last 7 days' },
    { value: '30d', label: 'Last 30 days' },
  ];

  const buildAuditFilters = (): FilterDef[] => [
    {
      id: 'audit-event',
      label: 'Event',
      group: 'properties',
      value: eventFilter,
      setValue: setEventFilter,
      defaultValue: '',
      options: auditEventOptions,
    },
    {
      id: 'audit-date-range',
      label: 'Time range',
      group: 'properties',
      value: dateRangeFilter,
      setValue: setDateRangeFilter,
      defaultValue: 'all',
      options: auditDateRangeOptions,
    },
    {
      id: 'audit-success',
      label: 'Outcome',
      group: 'status',
      value: successFilter,
      setValue: setSuccessFilter,
      defaultValue: 'all',
      options: auditSuccessOptions,
    },
    {
      id: 'audit-verification',
      label: 'Verification',
      group: 'status',
      value: verificationFilter,
      setValue: setVerificationFilter,
      defaultValue: 'all',
      options: auditVerificationOptions,
    },
  ];

  return (
    <SettingsPanel noPadding>
      <Show when={!showUpgradePaywall()}>
        <div class="flex flex-wrap items-center justify-between gap-2 border-b border-border bg-surface-alt px-3 py-3 sm:px-6 sm:py-4">
          <div class="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={refresh}
              disabled={!accessAllowed()}
              class={AUDIT_REFRESH_BUTTON_CLASS}
            >
              <RefreshCw aria-hidden="true" class="w-4 h-4" />
              {loading() && accessAllowed() ? 'Restart load' : 'Refresh'}
            </button>
            <button
              type="button"
              onClick={verifyAll}
              disabled={!isPersistent() || loading() || verifyingAll() || !hasSignedEvents()}
              class={AUDIT_VERIFY_ALL_BUTTON_CLASS}
            >
              {(() => {
                const presentation = getAuditEventStatusPresentation(true);
                const Icon = presentation.icon;
                return <Icon class="w-4 h-4" />;
              })()}
              {verifyAllLabel()}
            </button>
            <Show when={verifyingAll()}>
              <button
                type="button"
                onClick={cancelVerification}
                class={`${AUDIT_TOOLBAR_BUTTON_CLASS} text-muted`}
              >
                Cancel
              </button>
            </Show>
            <Show when={hasResumeEvents() && !verifyingAll()}>
              <button
                type="button"
                onClick={resumeVerification}
                class={`${AUDIT_TOOLBAR_BUTTON_CLASS} text-muted`}
                onMouseEnter={(e) => {
                  const rect = e.currentTarget.getBoundingClientRect();
                  showTooltip(
                    'Retries failed, error, or unchecked signatures on this page.',
                    rect.left + rect.width / 2,
                    rect.top,
                    { align: 'center', direction: 'up', maxWidth: 260 },
                  );
                }}
                onMouseLeave={() => hideTooltip()}
              >
                <Play class="w-4 h-4" />
                Resume{resumeCount() > 0 ? ` (${resumeCount()})` : ''}
              </button>
            </Show>
            <Show when={verifyCanceled() && !verifyingAll()}>
              <span class="text-xs text-amber-600 dark:text-amber-400">Verification canceled</span>
            </Show>
          </div>
          <Show when={isPersistent()}>
            <div class="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={exportCsv}
                disabled={loading() || filteredEvents().length === 0}
                class={`${AUDIT_TOOLBAR_BUTTON_CLASS} text-base-content`}
              >
                <Download aria-hidden="true" class="w-4 h-4" />
                Export CSV
              </button>
              <button
                type="button"
                onClick={exportJson}
                disabled={loading() || filteredEvents().length === 0}
                class={`${AUDIT_TOOLBAR_BUTTON_CLASS} text-base-content`}
              >
                <Download aria-hidden="true" class="w-4 h-4" />
                Export JSON
              </button>
            </div>
          </Show>
        </div>
      </Show>

      <div class="space-y-3 p-2.5 max-sm:pb-(--pulse-mobile-nav-height) sm:space-y-5 sm:p-6">
        <Show when={showUpgradePaywall()}>
          <div class="p-6 bg-surface-alt border border-border rounded-md">
            <div class="flex flex-col sm:flex-row items-center gap-4">
              <div class="flex-1">
                <h3 class="text-lg font-semibold text-base-content">{featureGateCopy().title}</h3>
                <p class="text-sm text-muted mt-1">{featureGateCopy().body}</p>
              </div>
              <Show when={showFeatureGateAction()}>
                <div class="flex flex-col items-center gap-2">
                  <UpgradeButtonLink
                    destination={upgradeDestination()}
                    mobileFullWidth={false}
                    tone={paidRuntimeRequired() ? 'warning' : 'primary'}
                  >
                    {upgradeActionLabel()}
                  </UpgradeButtonLink>
                </div>
              </Show>
            </div>
          </div>
        </Show>

        <Show when={showLogFilters()}>
          <div class="space-y-2.5">
            <FilterBar
              role="group"
              ariaLabel="Audit log filters"
              isMobile={isMobile}
              search={{
                value: userFilter,
                setValue: setUserFilter,
                placeholder: 'Filter by user...',
              }}
              filters={buildAuditFilters()}
              viewOptions={
                <div>
                  <div class="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted">
                    Pagination
                  </div>
                  <FormSelect
                    label="Audit page size"
                    labelClass="sr-only"
                    fieldBaseClass="contents"
                    value={String(pageSize())}
                    onChange={(e) => setPageSize(Number(e.currentTarget.value))}
                    selectBaseClass="min-h-9 w-full px-2.5 py-1.5 text-xs border border-border rounded-md bg-surface text-base-content"
                  >
                    <option value="25">25 / page</option>
                    <option value="50">50 / page</option>
                    <option value="100">100 / page</option>
                    <option value="200">200 / page</option>
                  </FormSelect>
                </div>
              }
              onClearAll={clearFilters}
              showClearAll={() => activeFilterCount() > 0}
            />
            <SearchField
              value={searchQuery()}
              onChange={setSearchQuery}
              placeholder="Search loaded events by IP, user, action, or details..."
              inputClass="min-h-10 sm:min-h-9 py-2"
            />
          </div>
        </Show>

        {/* Secondary collapsible Signature Verification controls */}
        <Show when={isPersistent()}>
          <details class="rounded-md border border-border bg-surface-alt/50 px-3 py-2.5">
            <summary class="flex cursor-pointer flex-wrap items-center justify-between gap-2 text-xs font-semibold text-base-content">
              <span>Signature verification settings</span>
              <span class="font-normal text-muted">
                Verified {verificationSummary().verified} / {verificationSummary().signed} signed
                {verificationSummary().failed + verificationSummary().error > 0
                  ? `, ${verificationSummary().failed + verificationSummary().error} failed`
                  : ''}
              </span>
            </summary>
            <div class="mt-3 space-y-3 border-t border-border pt-3">
              <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
                <Toggle
                  label="Auto-verify signatures"
                  description="Verify signatures after loading audit events."
                  checked={autoVerifyEnabled()}
                  onChange={(e) => setAutoVerifyEnabled(e.currentTarget.checked)}
                />
                <div class="flex flex-col gap-2 text-xs text-muted sm:flex-row sm:flex-wrap sm:items-center">
                  <div class="flex flex-wrap items-center gap-2">
                    <label class="flex items-center gap-2">
                      <span>Auto-verify limit</span>
                      <input
                        type="number"
                        min="0"
                        max="500"
                        value={autoVerifyLimit()}
                        disabled={!autoVerifyEnabled()}
                        onInput={(e) => {
                          const raw = Number(e.currentTarget.value);
                          if (!Number.isFinite(raw)) {
                            setAutoVerifyLimit(0);
                            return;
                          }
                          const clamped = Math.max(0, Math.min(500, Math.floor(raw)));
                          setAutoVerifyLimit(clamped);
                        }}
                        class="min-h-10 sm:min-h-10 w-24 rounded-md border border-border bg-surface px-2.5 py-2 text-sm text-base-content"
                      />
                    </label>
                    <HelpIcon
                      inline={{
                        title: 'Auto-verify limit',
                        description:
                          'Large verification batches can add load. Use a smaller limit on busy systems.',
                      }}
                      size="xs"
                    />
                  </div>
                  <span class="text-muted">0 disables, max 500</span>
                  <button
                    type="button"
                    onClick={resetPreferences}
                    class="w-full sm:w-auto sm:ml-2 min-h-10 sm:min-h-10 px-3 py-2 text-sm font-medium text-muted border border-border rounded-md hover:bg-surface-hover"
                  >
                    Reset preferences
                  </button>
                </div>
              </div>
            </div>
          </details>
        </Show>

        <Show when={!loading() && auditLoggingEnabled() && !accessAllowed()}>
          <InlineNotice tone="warning" role="status">
            Audit log is unavailable for this session.
          </InlineNotice>
        </Show>

        <Show when={verificationNotice()}>
          {(notice) => (
            <InlineNotice tone={notice().tone} role="status" aria-live="polite">
              {notice().message}
            </InlineNotice>
          )}
        </Show>

        <Show when={error()}>
          <div
            role="alert"
            aria-live="assertive"
            class="p-4 bg-red-50 dark:bg-red-900/25 border border-red-200 dark:border-red-800 rounded-md text-red-700 dark:text-red-300"
          >
            {error()}
          </div>
        </Show>

        <Show when={loading()}>
          <div role="status" class="flex flex-col items-center gap-2 py-12 text-sm text-muted">
            <RefreshCw aria-hidden="true" class="w-8 h-8 text-blue-500 motion-safe:animate-spin" />
            <span>{getAuditLogLoadingState().text}</span>
          </div>
        </Show>

        <Show when={!loading() && isPersistent() && loadedEventCount() > 0}>
          <div
            class="flex flex-wrap items-center gap-x-3 text-xs text-muted"
            classList={{
              'gap-y-0 sm:gap-y-3': hasPageVerificationFilter(),
              'gap-y-1 sm:gap-y-3': !hasPageVerificationFilter(),
            }}
          >
            <span>Total: {totalEvents()}</span>
            <span>Signed: {verificationSummary().signed}</span>
            <span>Verified: {verificationSummary().verified}</span>
            <span>Checks failed: {verificationSummary().failed}</span>
            <span>Check errors: {verificationSummary().error}</span>
            <span>Unavailable: {verificationSummary().unavailable}</span>
            <span>Unchecked: {verificationSummary().unchecked}</span>
            <HelpIcon
              triggerLabel="Legend"
              inline={{
                title: 'Audit signature verification',
                description: [
                  'Unsigned: no signature stored for this event',
                  'Unavailable: verification not supported by the logger',
                  'Not checked: signature exists but has not been verified',
                  'Check failed: the signature did not pass verification. The cause is unknown.',
                  'Check error: the check did not return a verification result',
                ].join('\n'),
              }}
              size="xs"
            />
            <span class="w-full sm:w-auto sm:ml-auto text-muted">
              {pageRangeText()} • Page {pageNumber()} of {totalPages()}
            </span>
            <Show when={hasPageVerificationFilter()}>
              <p role="status" aria-label="Verification filter results" class="w-full">
                Verification filters this page only: {filteredEvents().length} of{' '}
                {loadedEventCount()} events shown.
              </p>
            </Show>
          </div>
        </Show>

        <Show when={!loading() && isPersistent() && filteredEvents().length > 0}>
          <div class="sm:mt-4">
            <PulseDataGrid
              class="[&_th]:px-2 [&_td]:px-2 max-sm:[&_td]:py-1.5"
              data={filteredEvents()}
              columns={[
                {
                  key: 'success',
                  label: 'Outcome',
                  hiddenOnMobile: true,
                  render: (event) => {
                    const presentation = getAuditEventStatusPresentation(event.success);
                    const Icon = presentation.icon;
                    return (
                      <span class="inline-flex items-center gap-1.5 whitespace-nowrap text-xs">
                        <Icon class={presentation.className} aria-hidden="true" />
                        {auditEventOutcomeLabel(event.success)}
                      </span>
                    );
                  },
                },
                {
                  key: 'timestamp',
                  label: 'Timestamp',
                  hiddenOnMobile: true,
                  render: (event) => (
                    <span class="text-muted">{formatTimestamp(event.timestamp)}</span>
                  ),
                },
                {
                  key: 'event',
                  label: 'Event',
                  render: (event) => {
                    const isExpanded = () => Boolean(expandedEvents()[event.id]);
                    const hasExtraDetails = () => Boolean(event.details || event.path);
                    return (
                      <div class="min-w-0">
                        <span
                          class={`inline-flex items-center px-2 py-0.5 rounded-sm text-xs font-medium ${getAuditEventTypeBadgeClass(event.event)}`}
                        >
                          {event.event}
                        </span>
                        <div class="mt-0.5 whitespace-normal wrap-anywhere text-[11px] leading-tight text-muted sm:hidden">
                          {auditEventOutcomeLabel(event.success)} · {event.user || 'Unknown user'}
                          <time class="block" dateTime={event.timestamp}>
                            {formatTimestamp(event.timestamp)}
                          </time>
                          <Show when={event.ip}>
                            <span class="block font-mono">IP: {event.ip}</span>
                          </Show>
                          <Show when={hasExtraDetails()}>
                            <button
                              type="button"
                              aria-expanded={isExpanded()}
                              onClick={() => toggleExpandedEvent(event.id)}
                              class="mt-1 inline-flex items-center gap-1 font-medium text-blue-600 dark:text-blue-400"
                            >
                              <Show
                                when={isExpanded()}
                                fallback={<ChevronRight class="h-3 w-3" aria-hidden="true" />}
                              >
                                <ChevronDown class="h-3 w-3" aria-hidden="true" />
                              </Show>
                              {isExpanded() ? 'Hide details' : 'Show details'}
                            </button>
                            <Show when={isExpanded()}>
                              <div class="mt-1.5 space-y-1 rounded-sm border border-border bg-surface-alt p-2 text-xs text-base-content">
                                <Show when={event.path}>
                                  <div class="font-mono text-[11px] text-muted">{event.path}</div>
                                </Show>
                                <Show when={event.details}>
                                  <p class="whitespace-pre-wrap wrap-anywhere">{event.details}</p>
                                </Show>
                              </div>
                            </Show>
                          </Show>
                        </div>
                      </div>
                    );
                  },
                },
                {
                  key: 'user',
                  label: 'User',
                  hiddenOnMobile: true,
                  render: (event) => <span class="text-base-content">{event.user || '-'}</span>,
                },
                {
                  key: 'ip',
                  label: 'IP',
                  hiddenOnMobile: true,
                  render: (event) => (
                    <span class="text-muted font-mono text-xs">{event.ip || '-'}</span>
                  ),
                },
                {
                  key: 'details',
                  label: 'Details',
                  hiddenOnMobile: true,
                  render: (event) => {
                    const isExpanded = () => Boolean(expandedEvents()[event.id]);
                    const hasExpandable = () =>
                      Boolean(event.path || (event.details && event.details.length > 48));
                    return (
                      <div class="min-w-0 max-w-sm">
                        <Show
                          when={event.details || event.path}
                          fallback={<span class="text-muted">-</span>}
                        >
                          <div class="flex items-start gap-1.5">
                            <span
                              class={
                                isExpanded()
                                  ? 'whitespace-pre-wrap wrap-anywhere text-xs text-base-content'
                                  : 'truncate text-muted'
                              }
                              title={event.details || event.path || ''}
                            >
                              {event.details || event.path}
                            </span>
                            <Show when={hasExpandable()}>
                              <button
                                type="button"
                                aria-expanded={isExpanded()}
                                aria-label={
                                  isExpanded()
                                    ? `Collapse details for event ${event.id}`
                                    : `Expand details for event ${event.id}`
                                }
                                onClick={() => toggleExpandedEvent(event.id)}
                                class="shrink-0 text-xs font-medium text-blue-600 hover:underline dark:text-blue-400"
                              >
                                {isExpanded() ? 'Less' : 'More'}
                              </button>
                            </Show>
                          </div>
                          <Show when={isExpanded() && event.path && event.details}>
                            <div class="mt-1 font-mono text-[11px] text-muted wrap-anywhere">
                              Path: {event.path}
                            </div>
                          </Show>
                        </Show>
                      </div>
                    );
                  },
                },
                {
                  key: 'verification',
                  label: 'Verification',
                  render: (event) => {
                    if (!event.signature) {
                      return <span class="text-xs">Unsigned</span>;
                    }
                    const state = verification()[event.id];
                    const isVerifying = verifying()[event.id];
                    const badge = getAuditVerificationBadgePresentation(state);

                    return (
                      <div class="flex flex-wrap items-center gap-x-2 gap-y-1 sm:flex-nowrap">
                        <Show
                          when={isVerifying}
                          fallback={
                            <span
                              class={`inline-flex items-center px-2 py-0.5 rounded-sm text-xs font-medium ${badge.className}`}
                            >
                              {badge.label}
                            </span>
                          }
                        >
                          <span class="text-xs text-muted">Verifying…</span>
                        </Show>
                        <button
                          type="button"
                          aria-label={`Verify signature: ${event.event} by ${event.user || 'Unknown user'} at ${formatTimestamp(event.timestamp)} (Event ID: ${event.id})`}
                          onClick={() => void verifyEvent(event)}
                          disabled={isVerifying}
                          class={`${AUDIT_VERIFY_ROW_BUTTON_CLASS} max-sm:min-w-11 max-sm:justify-center max-sm:px-1.5`}
                        >
                          Verify
                        </button>
                      </div>
                    );
                  },
                },
              ]}
              keyExtractor={(event) => event.id}
              desktopMinWidth="900px"
            />
          </div>
        </Show>

        <Show when={!loading() && isPersistent() && filteredEvents().length === 0}>
          <Show
            when={hasPageVerificationFilter() && loadedEventCount() > 0}
            fallback={
              <EmptyState
                variant="panel"
                icon={
                  activeFilterCount() > 0 ? (
                    <ShieldAlert class="h-5 w-5" />
                  ) : (
                    <Shield class="h-5 w-5" />
                  )
                }
                title={logEmptyState().title}
                description={logEmptyState().description}
                tone={activeFilterCount() > 0 ? 'info' : 'default'}
                actions={
                  activeFilterCount() > 0 ? (
                    <Button type="button" variant="info" size="mdCompact" onClick={clearFilters}>
                      Clear all filters
                    </Button>
                  ) : undefined
                }
              />
            }
          >
            <InlineNotice
              tone="info"
              icon={<ShieldAlert />}
              actionLabel="Clear all filters"
              actionOnClick={clearFilters}
            >
              <h3 class="font-semibold">No events match Verification on this page</h3>
              <p>
                {totalPages() > 1
                  ? 'Check another page or change the Verification filter.'
                  : 'Change the Verification filter to see other events on this page.'}
              </p>
            </InlineNotice>
          </Show>
        </Show>

        <Show when={!loading() && isPersistent()}>
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-end">
            <div class="flex items-center gap-2 text-xs text-muted">
              <label class="flex items-center gap-2">
                <span>Page</span>
                <input
                  type="number"
                  min="1"
                  value={pageInput()}
                  onInput={(e) => setPageInput(e.currentTarget.value)}
                  onKeyDown={(e) => e.key === 'Enter' && submitPageInput()}
                  class="min-h-10 sm:min-h-10 w-20 rounded-md border border-border bg-surface px-2.5 text-sm text-base-content"
                />
              </label>
              <button
                type="button"
                onClick={submitPageInput}
                class="min-h-10 sm:min-h-10 px-3 py-2.5 text-sm font-medium text-base-content bg-surface border border-border rounded-md hover:bg-surface-hover"
              >
                Go
              </button>
            </div>
            <div class="grid grid-cols-4 gap-1.5 sm:flex sm:flex-wrap sm:items-center sm:justify-end sm:gap-2">
              <button
                type="button"
                onClick={goToFirstPage}
                disabled={pageNumber() === 1}
                class={AUDIT_PAGE_BUTTON_CLASS}
              >
                First
              </button>
              <button
                type="button"
                onClick={goToPreviousPage}
                disabled={pageNumber() === 1}
                class={AUDIT_PAGE_BUTTON_CLASS}
              >
                Previous
              </button>
              <button
                type="button"
                onClick={goToNextPage}
                disabled={!hasNextPage()}
                class={AUDIT_PAGE_BUTTON_CLASS}
              >
                Next
              </button>
              <button
                type="button"
                onClick={goToLastPage}
                disabled={!hasNextPage()}
                class={AUDIT_PAGE_BUTTON_CLASS}
              >
                Last
              </button>
            </div>
          </div>
        </Show>
      </div>
    </SettingsPanel>
  );
}
