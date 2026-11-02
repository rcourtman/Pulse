import { For, Show, type Component } from 'solid-js';
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
import SettingsPanel from '@/components/shared/SettingsPanel';
import { formControl } from '@/components/shared/Form';
import {
  AUDIT_WEBHOOK_ENDPOINT_CARD_CLASS,
  AUDIT_WEBHOOK_ENDPOINT_ICON_CLASS,
  AUDIT_WEBHOOK_READONLY_NOTICE_CLASS,
  AUDIT_WEBHOOK_SECURITY_NOTE_BODY,
  AUDIT_WEBHOOK_SECURITY_NOTE_TITLE,
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
  let newUrlInputRef: HTMLInputElement | undefined;
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
    hasLoadedConfiguration,
    newUrl,
    openRemoveWebhook,
    pendingRemoval,
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
          <SettingsPanel title="Audit Webhooks" noPadding bodyClass="divide-y divide-border">
            <div class="space-y-6 p-4 sm:p-6">
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
              <Show when={loadError()}>
                <Card tone="warning" padding="md">
                  <div role="alert" class="space-y-2 text-sm">
                    <p class="font-medium">{readErrorCopy().title}</p>
                    <p>{readErrorCopy().body}</p>
                    <Button onClick={() => void fetchWebhooks()} disabled={loading() || saving()}>
                      {readErrorCopy().retryLabel}
                    </Button>
                  </div>
                </Card>
              </Show>
              <Show
                when={!loading() || hasLoadedConfiguration()}
                fallback={<p class="text-sm text-muted">{getAuditWebhookLoadingState().text}</p>}
              >
                <Show when={loading()}>
                  <p class="text-sm text-muted" role="status">
                    {getAuditWebhookLoadingState().text}
                  </p>
                </Show>
                <div class="space-y-3">
                  <For each={webhookUrls()}>
                    {(url) => (
                      <div class={AUDIT_WEBHOOK_ENDPOINT_CARD_CLASS}>
                        <div class="flex items-center gap-3 overflow-hidden min-w-0">
                          <div class={AUDIT_WEBHOOK_ENDPOINT_ICON_CLASS}>
                            <ExternalLink size={16} />
                          </div>
                          <span class="text-sm font-medium text-base-content truncate" title={url}>
                            {url}
                          </span>
                        </div>
                        <button
                          onClick={() => openRemoveWebhook(url)}
                          disabled={!canEdit() || saving()}
                          class="p-2 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/25 rounded-md transition-colors"
                          title="Remove webhook endpoint"
                        >
                          <Trash2 size={16} />
                        </button>
                      </div>
                    )}
                  </For>

                  <Show
                    when={
                      hasLoadedConfiguration() &&
                      !loading() &&
                      !loadError() &&
                      webhookUrls().length === 0
                    }
                  >
                    <EmptyState
                      variant="panel"
                      icon={<Globe class="h-5 w-5" />}
                      title={emptyStateCopy().title}
                    />
                  </Show>
                </div>
              </Show>
              <div class="flex flex-col gap-3 pt-4 border-t border-border sm:flex-row">
                <input
                  ref={newUrlInputRef}
                  type="text"
                  aria-label="Audit webhook URL"
                  placeholder="https://your-api.com/webhook"
                  class={`${formControl} w-full min-w-0 flex-1`}
                  value={newUrl()}
                  onInput={(e) => setNewUrl(e.currentTarget.value)}
                  onKeyDown={(e) => e.key === 'Enter' && handleAddWebhook()}
                  disabled={!canManage()}
                />
                <button
                  onClick={handleAddWebhook}
                  disabled={!canEdit() || saving() || !newUrl().trim()}
                  class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-md flex items-center justify-center gap-2 transition-colors sm:shrink-0"
                >
                  <Plus size={18} />
                  Add Endpoint
                </button>
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
                returnFocus={() => newUrlInputRef}
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
                      disabled={!canEdit() || saving()}
                      onClick={() => void handleRemoveWebhook()}
                      class="px-4 py-2 text-sm font-medium bg-red-600 text-white rounded-md hover:bg-red-700"
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
