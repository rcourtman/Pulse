import { For, Show } from 'solid-js';
import Shield from 'lucide-solid/icons/shield';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import Filter from 'lucide-solid/icons/filter';
import Info from 'lucide-solid/icons/info';
import Play from 'lucide-solid/icons/play';
import X from 'lucide-solid/icons/x';
import ShieldAlert from 'lucide-solid/icons/shield-alert';
import { showTooltip, hideTooltip } from '@/components/shared/Tooltip';
import Toggle from '@/components/shared/Toggle';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import {
  AUDIT_REFRESH_BUTTON_CLASS,
  AUDIT_VERIFY_ALL_BUTTON_CLASS,
  AUDIT_VERIFY_ROW_BUTTON_CLASS,
  AUDIT_TOOLBAR_BUTTON_CLASS,
  getAuditEventStatusPresentation,
  getAuditEventTypeBadgeClass,
  getAuditLogEmptyState,
  getAuditLogLoadingState,
  getAuditVerificationBadgePresentation,
} from '@/utils/auditLogPresentation';
import {
  getUpgradeActionButtonClass,
  UPGRADE_ACTION_LABEL,
  UPGRADE_TRIAL_LABEL,
  UPGRADE_TRIAL_LINK_CLASS,
} from '@/utils/upgradePresentation';
import { useAuditLogPanelState } from '@/components/Settings/useAuditLogPanelState';

export default function AuditLogPanel() {
  const {
    activeFilterChips,
    activeFilterCount,
    applyFilters,
    auditLoggingEnabled,
    autoVerifyEnabled,
    autoVerifyLimit,
    canStartTrial,
    cancelVerification,
    clearFilterChip,
    clearFilters,
    error,
    eventFilter,
    filteredEvents,
    goToFirstPage,
    goToLastPage,
    goToNextPage,
    goToPreviousPage,
    handleStartTrial,
    handleUpgradeClick,
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
    setAutoVerifyEnabled,
    setAutoVerifyLimit,
    setEventFilter,
    setPageInput,
    setPageSize,
    setSuccessFilter,
    setUserFilter,
    setVerificationFilter,
    showUpgradePaywall,
    startingTrial,
    submitPageInput,
    successFilter,
    totalEvents,
    totalPages,
    upgradeActionUrl,
    verification,
    verificationFilter,
    verificationSummary,
    verifyAll,
    verifyAllLabel,
    verifyCanceled,
    verifyEvent,
    verifying,
    verifyingAll,
    userFilter,
  } = useAuditLogPanelState();

  const formatTimestamp = (ts: string) => {
    const date = new Date(ts);
    return date.toLocaleString();
  };

  return (
    <SettingsPanel
      title="Audit Log"
      description="Persistent, searchable audit events with optional signature verification."
      icon={<Shield class="w-5 h-5" strokeWidth={2} />}
      noPadding
      bodyClass="space-y-6 p-4 sm:p-6"
      action={
        <div class="flex w-full flex-wrap items-center gap-2 sm:w-auto">
          <button
            onClick={refresh}
            disabled={loading() || !auditLoggingEnabled()}
            class={AUDIT_REFRESH_BUTTON_CLASS}
          >
            <RefreshCw class={`w-4 h-4 ${loading() ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <button
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
          <button
            onClick={cancelVerification}
            disabled={!verifyingAll()}
            class={`${AUDIT_TOOLBAR_BUTTON_CLASS} text-muted`}
          >
            Cancel
          </button>
          <button
            onClick={resumeVerification}
            disabled={verifyingAll() || !hasResumeEvents()}
            class={`${AUDIT_TOOLBAR_BUTTON_CLASS} text-muted`}
            onMouseEnter={(e) => {
              if (!hasResumeEvents()) return;
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
          <Show when={verifyCanceled() && !verifyingAll()}>
            <span class="text-xs text-amber-600 dark:text-amber-400">Verification canceled</span>
          </Show>
        </div>
      }
    >
      <Show when={showUpgradePaywall()}>
          <div class="p-6 bg-surface-alt border border-border rounded-md">
            <div class="flex flex-col sm:flex-row items-center gap-4">
              <div class="flex-1">
                <h3 class="text-lg font-semibold text-base-content">Audit Logging</h3>
                <p class="text-sm text-muted mt-1">
                  Persistent, searchable audit logs with cryptographic signature verification.
                </p>
              </div>
              <div class="flex flex-col items-center gap-2">
                <a
                  href={upgradeActionUrl()}
                  target="_blank"
                  rel="noopener noreferrer"
                  class={getUpgradeActionButtonClass({ mobileFullWidth: false })}
                  onClick={handleUpgradeClick}
                >
                  {UPGRADE_ACTION_LABEL}
                </a>
                <Show when={canStartTrial()}>
                  <button
                    type="button"
                    onClick={handleStartTrial}
                    disabled={startingTrial()}
                    class={UPGRADE_TRIAL_LINK_CLASS}
                  >
                    {UPGRADE_TRIAL_LABEL}
                  </button>
                </Show>
              </div>
            </div>
          </div>
      </Show>

      <Show when={isPersistent()}>
          <div class="flex flex-wrap gap-3 p-4 bg-surface-alt rounded-md">
            <div class="flex items-center gap-2">
              <Filter class="w-4 h-4 text-slate-400" />
              <span class="text-sm font-medium text-base-content">Filters:</span>
            </div>
            <select
              value={eventFilter()}
              onChange={(e) => setEventFilter(e.currentTarget.value)}
              class="w-full sm:w-auto min-h-10 sm:min-h-10 px-3 py-2.5 text-sm border border-border rounded-md bg-surface text-base-content"
            >
              <option value="">All Events</option>
              <option value="login">Login</option>
              <option value="logout">Logout</option>
              <option value="config_change">Config Change</option>
              <option value="startup">Startup</option>
            </select>
            <input
              type="text"
              placeholder="Filter by user..."
              value={userFilter()}
              onInput={(e) => setUserFilter(e.currentTarget.value)}
              class="w-full sm:w-auto min-h-10 sm:min-h-10 px-3 py-2.5 text-sm border border-border rounded-md bg-surface text-base-content placeholder-gray-400"
            />
            <select
              value={successFilter()}
              onChange={(e) => setSuccessFilter(e.currentTarget.value)}
              class="w-full sm:w-auto min-h-10 sm:min-h-10 px-3 py-2.5 text-sm border border-border rounded-md bg-surface text-base-content"
            >
              <option value="all">All</option>
              <option value="success">Success Only</option>
              <option value="failed">Failed Only</option>
            </select>
            <select
              value={verificationFilter()}
              onChange={(e) => setVerificationFilter(e.currentTarget.value)}
              class="w-full sm:w-auto min-h-10 sm:min-h-10 px-3 py-2.5 text-sm border border-border rounded-md bg-surface text-base-content"
            >
              <option value="all">All Verification</option>
              <option value="needs">Needs Verification</option>
              <option value="verified">Verified</option>
              <option value="failed">Failed/Error</option>
            </select>
            <select
              value={String(pageSize())}
              onChange={(e) => setPageSize(Number(e.currentTarget.value))}
              class="w-full sm:w-auto min-h-10 sm:min-h-10 px-3 py-2.5 text-sm border border-border rounded-md bg-surface text-base-content"
            >
              <option value="25">25 / page</option>
              <option value="50">50 / page</option>
              <option value="100">100 / page</option>
              <option value="200">200 / page</option>
            </select>
            <button
              onClick={applyFilters}
              class="w-full sm:w-auto min-h-10 sm:min-h-10 px-3 py-2.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md"
            >
              Apply
            </button>
            <button
              onClick={clearFilters}
              class="w-full sm:w-auto min-h-10 sm:min-h-10 px-3 py-2.5 text-sm font-medium text-base-content bg-surface border border-border rounded-md hover:bg-surface-hover"
            >
              Clear{activeFilterCount() > 0 ? ` (${activeFilterCount()})` : ''}
            </button>
          </div>
          <Show when={activeFilterCount() > 0}>
            <div class="mt-2 flex flex-wrap gap-2">
              <For each={activeFilterChips()}>
                {(chip) => (
                  <button
                    type="button"
                    onClick={() => clearFilterChip(chip.key)}
                    class="inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium rounded-full bg-surface-alt text-base-content hover:bg-surface-hover"
                    title="Click to clear filter"
                    aria-label={`Clear ${chip.label}`}
                    onMouseEnter={(e) => {
                      const rect = e.currentTarget.getBoundingClientRect();
                      showTooltip('Click to clear filter', rect.left + rect.width / 2, rect.top, {
                        align: 'center',
                        direction: 'up',
                      });
                    }}
                    onMouseLeave={() => hideTooltip()}
                    onKeyDown={(e) => {
                      if (e.key !== 'Enter' && e.key !== ' ') return;
                      e.preventDefault();
                      clearFilterChip(chip.key);
                    }}
                  >
                    {chip.label}
                    <X class="w-3 h-3 text-muted" />
                  </button>
                )}
              </For>
            </div>
          </Show>
        </Show>

        {/* Verification Preferences */}
        <Show when={isPersistent()}>
          <div class="flex flex-col gap-4 px-1 lg:flex-row lg:items-center lg:justify-between">
            <Toggle
              label="Auto-verify signatures"
              description="Verify signatures after loading audit events."
              checked={autoVerifyEnabled()}
              onChange={(e) => setAutoVerifyEnabled(e.currentTarget.checked)}
            />
            <div class="flex flex-col gap-2 text-xs text-muted sm:flex-row sm:flex-wrap sm:items-center">
              <label class="flex items-center gap-2">
                <span>Auto-verify limit</span>
                <input
                  type="number"
                  min="0"
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
              <span class="text-muted">0 disables, max 500</span>
              <span
                class="inline-flex items-center cursor-help"
                onMouseEnter={(e) => {
                  const rect = e.currentTarget.getBoundingClientRect();
                  showTooltip(
                    'Large verification batches can add load. Use a smaller limit on busy systems.',
                    rect.left + rect.width / 2,
                    rect.top,
                    { align: 'center', direction: 'up', maxWidth: 260 },
                  );
                }}
                onMouseLeave={() => hideTooltip()}
              >
                <Info class="w-3 h-3" />
              </span>
              <button
                onClick={resetPreferences}
                class="w-full sm:w-auto sm:ml-2 min-h-10 sm:min-h-10 px-3 py-2 text-sm font-medium text-muted border border-border rounded-md hover:bg-surface-hover"
              >
                Reset preferences
              </button>
            </div>
          </div>
      </Show>

      <Show when={error()}>
          <div class="p-4 bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800 rounded-md text-red-700 dark:text-red-300">
            {error()}
          </div>
      </Show>

      <Show when={loading()}>
          <div class="flex items-center justify-center py-12">
            <RefreshCw class="w-8 h-8 text-blue-500 animate-spin" />
            <span class="sr-only">{getAuditLogLoadingState().text}</span>
          </div>
      </Show>

      <Show when={!loading() && isPersistent() && filteredEvents().length > 0}>
          <div class="flex flex-wrap items-center gap-3 text-xs text-muted">
            <span>Total: {totalEvents()}</span>
            <span>Signed: {verificationSummary().signed}</span>
            <span>Verified: {verificationSummary().verified}</span>
            <span>Failed: {verificationSummary().failed}</span>
            <span>Errors: {verificationSummary().error}</span>
            <span>Unavailable: {verificationSummary().unavailable}</span>
            <span>Unchecked: {verificationSummary().unchecked}</span>
            <span
              class="inline-flex items-center gap-1 text-muted cursor-help"
              onMouseEnter={(e) => {
                const rect = e.currentTarget.getBoundingClientRect();
                const content = [
                  'Unsigned: no signature stored for this event',
                  'Unavailable: verification not supported by the logger',
                  'Not checked: signature exists but has not been verified',
                  'Failed: signature mismatch or tampering detected',
                  'Error: verification request failed',
                ].join('\n');
                showTooltip(content, rect.left + rect.width / 2, rect.top, {
                  align: 'center',
                  direction: 'up',
                  maxWidth: 320,
                });
              }}
              onMouseLeave={() => hideTooltip()}
            >
              <Info class="w-3 h-3" />
              Legend
            </span>
            <span class="w-full sm:w-auto sm:ml-auto text-muted">
              {pageRangeText()} • Page {pageNumber()} of {totalPages()}
            </span>
          </div>
          <div class="mt-4">
            <PulseDataGrid
              data={filteredEvents()}
              columns={[
                {
                  key: 'success',
                  label: 'Status',
                  width: '64px',
                  align: 'center',
                  render: (event) => {
                    const presentation = getAuditEventStatusPresentation(event.success);
                    const Icon = presentation.icon;
                    return <Icon class={presentation.className} />;
                  },
                },
                {
                  key: 'timestamp',
                  label: 'Timestamp',
                  render: (event) => (
                    <span class="text-muted">{formatTimestamp(event.timestamp)}</span>
                  ),
                },
                {
                  key: 'event',
                  label: 'Event',
                  render: (event) => (
                    <span
                      class={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${getAuditEventTypeBadgeClass(event.event)}`}
                    >
                      {event.event}
                    </span>
                  ),
                },
                {
                  key: 'user',
                  label: 'User',
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
                  render: (event) => (
                    <span class="text-muted truncate max-w-xs">{event.details || '-'}</span>
                  ),
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
                      <div class="flex items-center gap-2">
                        <Show
                          when={isVerifying}
                          fallback={
                            <span
                              class={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${badge.className}`}
                            >
                              {badge.label}
                            </span>
                          }
                        >
                          <span class="text-xs text-muted">Verifying…</span>
                        </Show>
                        <button
                          onClick={() => void verifyEvent(event)}
                          disabled={isVerifying}
                          class={AUDIT_VERIFY_ROW_BUTTON_CLASS}
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
          <div class="text-center py-12 px-4 bg-surface-alt rounded-md border border-dashed border-border">
            <div class="flex flex-col items-center max-w-sm mx-auto">
              <Show
                when={activeFilterCount() > 0}
                fallback={<Shield class="w-12 h-12 text-slate-300 mb-4" />}
              >
                <ShieldAlert class="w-12 h-12 text-blue-300 dark:text-blue-900 mb-4" />
              </Show>
              <h3 class="text-lg font-medium text-base-content">
                {getAuditLogEmptyState(activeFilterCount()).title}
              </h3>
              <p class="mt-2 text-sm text-muted">
                {getAuditLogEmptyState(activeFilterCount()).description}
              </p>
              <Show when={activeFilterCount() > 0}>
                <button
                  onClick={clearFilters}
                  class="mt-6 px-4 py-2 text-sm font-medium text-blue-600 dark:text-blue-400 border border-blue-200 dark:border-blue-800 rounded-md hover:bg-blue-50 dark:hover:bg-blue-900"
                >
                  Clear all filters
                </button>
              </Show>
            </div>
          </div>
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
                onClick={submitPageInput}
                class="min-h-10 sm:min-h-10 px-3 py-2.5 text-sm font-medium text-base-content bg-surface border border-border rounded-md hover:bg-surface-hover"
              >
                Go
              </button>
            </div>
            <div class="grid grid-cols-2 gap-2 sm:flex sm:flex-wrap sm:items-center sm:justify-end">
              <button
                onClick={goToFirstPage}
                disabled={pageNumber() === 1}
                class="min-h-10 sm:min-h-10 px-3 py-2.5 text-sm font-medium text-base-content bg-surface border border-border rounded-md hover:bg-surface-hover disabled:opacity-50"
              >
                First
              </button>
              <button
                onClick={goToPreviousPage}
                disabled={pageNumber() === 1}
                class="min-h-10 sm:min-h-10 px-3 py-2.5 text-sm font-medium text-base-content bg-surface border border-border rounded-md hover:bg-surface-hover disabled:opacity-50"
              >
                Previous
              </button>
              <button
                onClick={goToNextPage}
                disabled={!hasNextPage()}
                class="min-h-10 sm:min-h-10 px-3 py-2.5 text-sm font-medium text-base-content bg-surface border border-border rounded-md hover:bg-surface-hover disabled:opacity-50"
              >
                Next
              </button>
              <button
                onClick={goToLastPage}
                disabled={!hasNextPage()}
                class="min-h-10 sm:min-h-10 px-3 py-2.5 text-sm font-medium text-base-content bg-surface border border-border rounded-md hover:bg-surface-hover disabled:opacity-50"
              >
                Last
              </button>
            </div>
          </div>
      </Show>
    </SettingsPanel>
  );
}
