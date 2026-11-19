import { For, Show, onCleanup, type Component } from 'solid-js';
import Shield from 'lucide-solid/icons/shield';
import Globe from 'lucide-solid/icons/globe';
import Plus from 'lucide-solid/icons/plus';
import Trash2 from 'lucide-solid/icons/trash-2';
import ExternalLink from 'lucide-solid/icons/external-link';
import { Card } from '@/components/shared/Card';
import { Button } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { EmptyState } from '@/components/shared/EmptyState';
import { FeatureGateSection } from '@/components/shared/FeatureGateSection';
import { InlineNotice } from '@/components/shared/InlineNotice';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { formControl } from '@/components/shared/Form';
import {
  AUDIT_WEBHOOK_CHECK_SAVED_LABEL,
  AUDIT_WEBHOOK_ENDPOINT_CARD_CLASS,
  AUDIT_WEBHOOK_ENDPOINT_ICON_CLASS,
  AUDIT_WEBHOOK_READONLY_NOTICE_CLASS,
  AUDIT_WEBHOOK_SECURITY_NOTE_BODY,
  AUDIT_WEBHOOK_SECURITY_NOTE_TITLE,
  AUDIT_WEBHOOK_UNCONFIRMED_MESSAGE,
  getAuditWebhookEmptyStateCopy,
  getAuditWebhookFeatureGateCopy,
  getAuditWebhookLoadingState,
  getAuditWebhookNotificationsSignpost,
  getAuditWebhookReadErrorCopy,
} from '@/utils/auditWebhookPresentation';
import { pathForTab } from '@/features/alerts/types';
import {
  useAuditWebhookPanelState,
  type AuditWebhookPanelProps,
} from '@/components/Settings/useAuditWebhookPanelState';

const AuditWebhookNotificationsSignpost: Component = () => {
  // A plain anchor: the app Router intercepts same-origin links, and the
  // panel also renders outside a Router in its read-only tests.
  const copy = getAuditWebhookNotificationsSignpost();
  return (
    <p class="text-sm text-muted leading-relaxed" data-audit-webhook-notifications-signpost>
      {copy.text}{' '}
      <a
        href={pathForTab('destinations')}
        class="font-medium text-blue-600 hover:underline dark:text-blue-400"
      >
        {copy.linkLabel}
      </a>
      .
    </p>
  );
};

export const AuditWebhookPanel: Component<AuditWebhookPanelProps> = (props) => {
  let configurationPanelRef: HTMLDivElement | undefined;
  let newUrlInputRef: HTMLInputElement | undefined;
  let endpointListRef: HTMLDivElement | undefined;
  let removalTrigger: HTMLButtonElement | undefined;
  let removalIndex = 0;
  const {
    actionError,
    canEdit,
    canRead,
    closeRemoveWebhook,
    canManage,
    fetchWebhooks,
    handleAddWebhook,
    handleRemoveWebhook,
    isAuditLoggingEnabled,
    loading,
    loadError,
    unconfirmedChange,
    hasLoadedConfiguration,
    newUrl,
    openRemoveWebhook,
    pendingRemoval,
    restartWebhooksRead,
    saving,
    setNewUrl,
    successNotice,
    showUpgradePrompts,
    showFeatureGateAction,
    upgradeDestination,
    upgradeActionLabel,
    paidRuntimeRequired,
    webhookUrls,
  } = useAuditWebhookPanelState(props);
  const retryConfigurationRead = () => {
    // The read removes its retry button. Keep a stable local position now,
    // rather than moving focus after a response when the user may have left.
    configurationPanelRef?.focus({ preventScroll: true });
    void fetchWebhooks();
  };
  const checkSavedDestinations = () => {
    // A check started in the dialog closes it and releases the inert
    // background synchronously. Keep focus local before any response arrives.
    void fetchWebhooks();
    configurationPanelRef?.focus({ preventScroll: true });
  };
  const restartConfigurationRead = () => {
    configurationPanelRef?.focus({ preventScroll: true });
    void restartWebhooksRead();
  };
  const ReadRecoveryButton: Component<{ savedCheck?: boolean }> = (buttonProps) => {
    let button: HTMLButtonElement | undefined;
    onCleanup(() => {
      // A result may arrive before a focused reader activates Restart.
      if (document.activeElement === button && canRead() && configurationPanelRef?.isConnected)
        configurationPanelRef.focus({ preventScroll: true });
    });
    return (
      <Button
        ref={button}
        variant="outline"
        size="sm"
        class="min-h-11 min-w-0 px-2 whitespace-normal sm:min-h-0 sm:shrink-0 sm:px-3"
        onClick={() => {
          if (!buttonProps.savedCheck || loading()) restartConfigurationRead();
          else checkSavedDestinations();
        }}
      >
        {buttonProps.savedCheck
          ? loading()
            ? 'Restart check'
            : AUDIT_WEBHOOK_CHECK_SAVED_LABEL
          : 'Restart loading'}
      </Button>
    );
  };
  const removalReturnTarget = () => {
    // A lost reply keeps the confirmation local but not actionable. On
    // dismissal or readback, continue from the surviving configuration region.
    if (unconfirmedChange()) return configurationPanelRef;
    // A withdrawn reader must not move focus into a different saved context.
    if (!canEdit()) return;
    if (removalTrigger?.isConnected && !removalTrigger.disabled) return removalTrigger;
    // A confirmed removal destroys its trigger. Keep the same list position,
    // or the preceding endpoint when the last one was removed.
    const buttons = endpointListRef?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)');
    return buttons?.[Math.min(removalIndex, buttons.length - 1)] ?? newUrlInputRef;
  };
  const featureGateCopy = () =>
    getAuditWebhookFeatureGateCopy({
      showCommercialCopy: showUpgradePrompts(),
      paidRuntimeRequired: paidRuntimeRequired(),
    });
  const emptyStateCopy = () => getAuditWebhookEmptyStateCopy();
  const readErrorCopy = () => getAuditWebhookReadErrorCopy(hasLoadedConfiguration());

  const featureGate = () => (
    <SettingsPanel title="Audit Webhooks">
      <Show when={!loading()} fallback={<div class="text-sm text-muted">Loading...</div>}>
        <AuditWebhookNotificationsSignpost />
        <Card tone="info" padding="md" class="mt-3">
          <FeatureGateSection
            title={featureGateCopy().title}
            body={featureGateCopy().body}
            upgradeDestination={upgradeDestination()}
            showUpgradePrompts={showFeatureGateAction()}
            upgradeLabel={upgradeActionLabel()}
            upgradeButtonTone={paidRuntimeRequired() ? 'warning' : 'primary'}
          />
        </Card>
      </Show>
    </SettingsPanel>
  );

  return (
    <Show when={isAuditLoggingEnabled()} fallback={featureGate()}>
      <Show
        when={canRead()}
        fallback={
          <SettingsPanel title="Audit Webhooks">
            <p role="status" class="text-sm text-muted">
              Audit webhook access is unavailable for this account.
            </p>
          </SettingsPanel>
        }
      >
        <div class="space-y-6">
          <SettingsPanel
            ref={configurationPanelRef}
            title="Audit Webhooks"
            role="region"
            aria-label="Audit webhook configuration"
            tabIndex={-1}
            class="focus:outline-2 focus:-outline-offset-2 focus:outline-blue-500 dark:focus:outline-blue-400"
            noPadding
            bodyClass="divide-y divide-border"
          >
            <div
              class="p-3 sm:space-y-6 sm:p-6"
              classList={{ 'space-y-2': unconfirmedChange(), 'space-y-4': !unconfirmedChange() }}
            >
              <Show when={!canManage()}>
                <div class={AUDIT_WEBHOOK_READONLY_NOTICE_CLASS}>
                  Audit webhook configuration is read-only for this account.
                </div>
              </Show>

              <p class="text-sm text-muted leading-relaxed">
                Pulse can send a signed event payload whenever security-relevant activity occurs
                (logins, settings changes, RBAC updates, and similar audit events).
              </p>
              <AuditWebhookNotificationsSignpost />

              <Show when={successNotice()}>
                <p role="status" class="text-sm text-green-700 dark:text-green-300">
                  {successNotice()}
                </p>
              </Show>
              <Show when={actionError() && !pendingRemoval()}>
                <p role="alert" class="text-sm text-red-700 dark:text-red-300">
                  {actionError()}
                </p>
              </Show>
              <Show when={unconfirmedChange() && !pendingRemoval()}>
                <p role="alert" class="text-sm text-amber-800 dark:text-amber-200">
                  <strong>{AUDIT_WEBHOOK_UNCONFIRMED_MESSAGE}</strong>{' '}
                  {loadError() ? 'Check failed. Last read shown.' : 'Showing the last read.'}
                </p>
              </Show>
              <Show when={loadError() && !unconfirmedChange()}>
                <Card tone="warning" padding="md">
                  <div role="alert" class="space-y-2 text-sm">
                    <p class="font-medium">{readErrorCopy().title}</p>
                    <p>{readErrorCopy().body}</p>
                    <Button onClick={retryConfigurationRead} disabled={loading() || saving()}>
                      {readErrorCopy().retryLabel}
                    </Button>
                  </div>
                </Card>
              </Show>
              <Show when={loading() && !unconfirmedChange()}>
                <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
                  <p class="text-sm text-muted" role="status">
                    {getAuditWebhookLoadingState().text}
                  </p>
                  <ReadRecoveryButton />
                </div>
              </Show>
              <Show when={hasLoadedConfiguration()}>
                <div
                  ref={endpointListRef}
                  role="group"
                  aria-label="Saved audit webhook destinations"
                  aria-busy={loading()}
                  class="space-y-2 sm:space-y-3"
                >
                  <For each={webhookUrls()}>
                    {(url) => (
                      <div class={AUDIT_WEBHOOK_ENDPOINT_CARD_CLASS}>
                        <div class="flex min-w-0 flex-1 items-center gap-3">
                          <div class={AUDIT_WEBHOOK_ENDPOINT_ICON_CLASS}>
                            <ExternalLink size={16} />
                          </div>
                          <span
                            class="min-w-0 text-sm font-medium text-base-content whitespace-normal [overflow-wrap:anywhere]"
                            title={url}
                          >
                            {url}
                          </span>
                        </div>
                        <button
                          type="button"
                          aria-label={`Remove webhook endpoint ${url}`}
                          onClick={(event) => {
                            removalTrigger = event.currentTarget;
                            removalIndex = webhookUrls().indexOf(url);
                            openRemoveWebhook(url);
                          }}
                          disabled={!canEdit() || saving()}
                          class="min-h-11 min-w-11 shrink-0 p-2 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/25 rounded-md transition-colors sm:min-h-0 sm:min-w-0"
                          title="Remove webhook endpoint"
                        >
                          <Trash2 size={16} />
                        </button>
                      </div>
                    )}
                  </For>

                  <Show when={!loading() && !loadError() && webhookUrls().length === 0}>
                    <EmptyState
                      variant="panel"
                      icon={<Globe class="h-5 w-5" />}
                      title={emptyStateCopy().title}
                    />
                  </Show>
                </div>
              </Show>
              <div
                class="border-t border-border pt-4 sm:flex sm:flex-row sm:gap-3"
                classList={{
                  'grid grid-cols-2 gap-2': unconfirmedChange(),
                  'flex flex-col gap-3': !unconfirmedChange(),
                }}
              >
                <input
                  ref={newUrlInputRef}
                  type="text"
                  aria-label="Audit webhook URL"
                  placeholder="https://your-api.com/webhook"
                  class={`${formControl} col-span-2 w-full min-w-0 flex-1`}
                  value={newUrl()}
                  onInput={(e) => setNewUrl(e.currentTarget.value)}
                  onKeyDown={(e) => e.key === 'Enter' && handleAddWebhook()}
                  disabled={!canManage()}
                />
                <button
                  onClick={handleAddWebhook}
                  disabled={!canEdit() || saving() || !newUrl().trim()}
                  class="flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50 sm:shrink-0"
                >
                  <Plus size={18} />
                  Add Endpoint
                </button>
                <Show when={unconfirmedChange() && !pendingRemoval()}>
                  <ReadRecoveryButton savedCheck />
                </Show>
              </div>
            </div>
          </SettingsPanel>
          <Card tone="warning" class="border border-amber-200 dark:border-amber-800">
            <div class="p-5 flex gap-4">
              <div class="p-3 bg-amber-100 dark:bg-amber-900/25 rounded-md h-fit text-amber-600 dark:text-amber-300">
                <Shield size={22} />
              </div>
              <div>
                <h3 class="text-base font-semibold text-amber-900 dark:text-amber-100 mb-1.5">
                  {AUDIT_WEBHOOK_SECURITY_NOTE_TITLE}
                </h3>
                <p class="text-sm text-amber-800 dark:text-amber-200 leading-relaxed">
                  {AUDIT_WEBHOOK_SECURITY_NOTE_BODY}
                </p>
              </div>
            </div>
          </Card>

          {/* Remove confirmation modal — webhook removal stops audit
          deliveries to that endpoint, so guard the action behind
          an explicit confirm. */}
          <Show when={pendingRemoval()} keyed>
            {(url) => (
              <Dialog
                isOpen={true}
                onClose={closeRemoveWebhook}
                panelClass="max-w-md"
                ariaLabel="Remove audit webhook"
                returnFocus={removalReturnTarget}
              >
                <div class="w-full p-6">
                  <h3 class="text-lg font-semibold text-base-content mb-2">
                    Remove audit webhook?
                  </h3>
                  <p class="text-sm text-muted mb-4">
                    This stops audit events from being delivered to{' '}
                    <span class="font-medium text-base-content break-all">{url}</span>. Any external
                    receiver using this URL will stop receiving Pulse audit traffic. You can add the
                    URL back later if needed.
                  </p>
                  <Show when={actionError()}>
                    <p role="alert" class="mb-4 text-sm text-red-700 dark:text-red-300">
                      {actionError()}
                    </p>
                  </Show>
                  <Show when={unconfirmedChange()}>
                    <InlineNotice
                      class="mb-4"
                      actionLabel={AUDIT_WEBHOOK_CHECK_SAVED_LABEL}
                      actionOnClick={checkSavedDestinations}
                    >
                      <p role="alert">
                        <strong>{AUDIT_WEBHOOK_UNCONFIRMED_MESSAGE}</strong> Check what’s saved
                        before removing a destination again.
                      </p>
                    </InlineNotice>
                  </Show>
                  <div class="flex justify-end gap-3">
                    <button
                      type="button"
                      disabled={saving()}
                      onClick={closeRemoveWebhook}
                      class="px-4 py-2 text-sm font-medium text-base-content border border-border rounded-md hover:bg-surface-hover"
                    >
                      Cancel
                    </button>
                    <button
                      type="button"
                      aria-disabled={!canEdit()}
                      onClick={() => void handleRemoveWebhook()}
                      class="px-4 py-2 text-sm font-medium bg-red-600 text-white rounded-md hover:bg-red-700 aria-disabled:cursor-not-allowed aria-disabled:opacity-50"
                    >
                      Remove webhook
                    </button>
                  </div>
                </div>
              </Dialog>
            )}
          </Show>
        </div>
      </Show>
    </Show>
  );
};
