import { For, Show, createSignal, type Component } from 'solid-js';
import Shield from 'lucide-solid/icons/shield';
import Globe from 'lucide-solid/icons/globe';
import Plus from 'lucide-solid/icons/plus';
import Trash2 from 'lucide-solid/icons/trash-2';
import ExternalLink from 'lucide-solid/icons/external-link';
import { Card } from '@/components/shared/Card';
import { Dialog } from '@/components/shared/Dialog';
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
} from '@/utils/auditWebhookPresentation';
import { useAuditWebhookPanelState } from '@/components/Settings/useAuditWebhookPanelState';

interface AuditWebhookPanelProps {
  canManage?: boolean;
}

export const AuditWebhookPanel: Component<AuditWebhookPanelProps> = (props) => {
  const [urlToRemove, setUrlToRemove] = createSignal<string | null>(null);
  const {
    canManage,
    handleAddWebhook,
    handleRemoveWebhook,
    isAuditLoggingEnabled,
    loading,
    newUrl,
    saving,
    setNewUrl,
    showUpgradePrompts,
    showFeatureGateAction,
    upgradeDestination,
    upgradeActionLabel,
    paidRuntimeRequired,
    webhookUrls,
  } = useAuditWebhookPanelState(props.canManage);
  const featureGateCopy = () =>
    getAuditWebhookFeatureGateCopy({
      showCommercialCopy: showUpgradePrompts(),
      paidRuntimeRequired: paidRuntimeRequired(),
    });
  const emptyStateCopy = () => getAuditWebhookEmptyStateCopy();

  if (!isAuditLoggingEnabled()) {
    return (
      <SettingsPanel title="Audit Webhooks">
        <Show when={!loading()} fallback={<div class="text-sm text-muted">Loading...</div>}>
          <Card tone="info" padding="md">
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
  }

  return (
    <div class="space-y-6">
      <SettingsPanel
        title="Audit Webhooks"
        noPadding
        bodyClass="divide-y divide-border"
      >
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

          <Show
            when={!loading()}
            fallback={<p class="text-sm text-muted">{getAuditWebhookLoadingState().text}</p>}
          >
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
                      onClick={() => setUrlToRemove(url)}
                      disabled={!canManage()}
                      class="p-2 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900 rounded-md transition-colors"
                      title="Remove webhook endpoint"
                    >
                      <Trash2 size={16} />
                    </button>
                  </div>
                )}
              </For>

              <Show when={webhookUrls().length === 0}>
                <div class="py-10 flex flex-col items-center justify-center text-muted border-2 border-dashed border-border rounded-md">
                  <Globe size={36} class="opacity-40 mb-3" />
                  <p class="text-sm">{emptyStateCopy().title}</p>
                </div>
              </Show>
            </div>
          </Show>
          <div class="flex gap-3 pt-4 border-t border-border">
            <input
              type="text"
              placeholder="https://your-api.com/webhook"
              class={`${formControl} flex-1`}
              value={newUrl()}
              onInput={(e) => setNewUrl(e.currentTarget.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAddWebhook()}
              disabled={!canManage()}
            />
            <button
              onClick={handleAddWebhook}
              disabled={!canManage() || saving() || !newUrl().trim()}
              class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-md flex items-center gap-2 transition-colors"
            >
              <Plus size={18} />
              Add Endpoint
            </button>
          </div>
        </div>
      </SettingsPanel>
      <Card tone="warning" class="border border-amber-200 dark:border-amber-800">
        <div class="p-5 flex gap-4">
          <div class="p-3 bg-amber-100 dark:bg-amber-900 rounded-md h-fit text-amber-600 dark:text-amber-300">
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
      <Show when={urlToRemove()}>
        <Dialog
          isOpen={true}
          onClose={() => setUrlToRemove(null)}
          panelClass="max-w-md"
          ariaLabel="Remove audit webhook"
        >
          <div class="w-full p-6">
            <h3 class="text-lg font-semibold text-base-content mb-2">Remove audit webhook?</h3>
            <p class="text-sm text-muted mb-4">
              This stops audit events from being delivered to{' '}
              <span class="font-medium text-base-content break-all">{urlToRemove()}</span>. Any
              external receiver using this URL will stop receiving Pulse audit traffic. You can
              add the URL back later if needed.
            </p>
            <div class="flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setUrlToRemove(null)}
                class="px-4 py-2 text-sm font-medium text-base-content border border-border rounded-md hover:bg-surface-hover"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => {
                  const url = urlToRemove();
                  setUrlToRemove(null);
                  if (url) void handleRemoveWebhook(url);
                }}
                class="px-4 py-2 text-sm font-medium bg-red-600 text-white rounded-md hover:bg-red-700"
              >
                Remove webhook
              </button>
            </div>
          </div>
        </Dialog>
      </Show>
    </div>
  );
};
