import { createSignal, For, onMount, Show, createEffect, type Component } from 'solid-js';
import Shield from 'lucide-solid/icons/shield';
import Globe from 'lucide-solid/icons/globe';
import Plus from 'lucide-solid/icons/plus';
import Trash2 from 'lucide-solid/icons/trash-2';
import ExternalLink from 'lucide-solid/icons/external-link';
import { Card } from '@/components/shared/Card';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { formControl } from '@/components/shared/Form';
import { showSuccess, showWarning } from '@/utils/toast';
import { apiFetchJSON } from '@/utils/apiClient';
import {
  hasFeature,
  licenseLoaded,
  loadLicenseStatus,
  getUpgradeActionUrlOrFallback,
  startProTrial,
  entitlements,
} from '@/stores/license';
import { trackPaywallViewed, trackUpgradeClicked } from '@/utils/upgradeMetrics';

interface AuditWebhookPanelProps {
  canManage?: boolean;
}

export const AuditWebhookPanel: Component<AuditWebhookPanelProps> = (props) => {
  const [webhookUrls, setWebhookUrls] = createSignal<string[]>([]);
  const [newUrl, setNewUrl] = createSignal('');
  const [saving, setSaving] = createSignal(false);
  const [loading, setLoading] = createSignal(true);
  const [startingTrial, setStartingTrial] = createSignal(false);
  const canManage = () => props.canManage !== false;

  const canStartTrial = () => entitlements()?.trial_eligible !== false;

  const handleStartTrial = async () => {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      const result = await startProTrial();
      if (result?.outcome === 'redirect') {
        window.location.href = result.actionUrl;
        return;
      }
      showSuccess('Pro trial started');
    } catch (err) {
      const statusCode = (err as { status?: number } | null)?.status;
      if (statusCode === 409) {
        showWarning('Trial already used');
      } else {
        showWarning(err instanceof Error ? err.message : 'Failed to start trial');
      }
    } finally {
      setStartingTrial(false);
    }
  };

  onMount(() => {
    loadLicenseStatus();
  });

  createEffect((wasPaywallVisible: boolean) => {
    const isPaywallVisible = licenseLoaded() && !hasFeature('audit_logging');
    if (isPaywallVisible && !wasPaywallVisible) {
      trackPaywallViewed('audit_logging', 'settings_audit_webhook_panel');
    }
    return isPaywallVisible;
  }, false);

  createEffect(() => {
    if (hasFeature('audit_logging')) {
      fetchWebhooks();
    } else {
      setLoading(false);
    }
  });

  const fetchWebhooks = async () => {
    try {
      const data = await apiFetchJSON<{ urls: string[] }>('/api/admin/webhooks/audit');
      setWebhookUrls(data.urls || []);
    } catch (err) {
      console.error('Failed to fetch audit webhooks:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleAddWebhook = async () => {
    if (!canManage()) return;
    const url = newUrl().trim();
    if (!url) return;

    try {
      new URL(url); // basic validation
    } catch {
      showWarning('Please enter a valid URL');
      return;
    }

    if (webhookUrls().includes(url)) {
      showWarning('This URL is already configured');
      return;
    }

    const updated = [...webhookUrls(), url];
    await saveWebhooks(updated);
    setNewUrl('');
  };

  const handleRemoveWebhook = async (url: string) => {
    if (!canManage()) return;
    const updated = webhookUrls().filter((u) => u !== url);
    await saveWebhooks(updated);
  };

  const saveWebhooks = async (urls: string[]) => {
    setSaving(true);
    try {
      await apiFetchJSON('/api/admin/webhooks/audit', {
        method: 'POST',
        body: JSON.stringify({ urls }),
      });

      setWebhookUrls(urls);
      showSuccess('Audit webhooks updated');
    } catch (_err) {
      showWarning('Failed to save webhook configuration');
    } finally {
      setSaving(false);
    }
  };

  if (!hasFeature('audit_logging')) {
    return (
      <SettingsPanel
        title="Audit Webhooks"
        description="Configure real-time delivery of security audit events to external systems."
        icon={<Globe class="w-5 h-5" strokeWidth={2} />}
      >
        <Show when={!loading()} fallback={<div class="text-sm text-muted">Loading...</div>}>
          <Card tone="info" padding="md">
            <div class="flex flex-col sm:flex-row items-center gap-4">
              <div class="flex-1 text-center sm:text-left">
                <p class="font-semibold text-base-content">Audit Webhooks (Pro)</p>
                <p class="mt-1 text-sm text-muted">
                  Audit webhooks are part of the audit logging feature set and require Pro.
                </p>
              </div>
              <div class="flex flex-col sm:flex-row items-center gap-2">
                <a
                  href={getUpgradeActionUrlOrFallback('audit_logging')}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="w-full sm:w-auto min-h-10 text-center sm:min-h-9 px-5 py-2.5 text-sm font-semibold bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors"
                  onClick={() =>
                    trackUpgradeClicked('settings_audit_webhook_panel', 'audit_logging')
                  }
                >
                  Upgrade to Pro
                </a>
                <Show when={canStartTrial()}>
                  <button
                    type="button"
                    onClick={handleStartTrial}
                    disabled={startingTrial()}
                    class="text-sm text-indigo-500 hover:underline disabled:opacity-50"
                  >
                    Start free trial
                  </button>
                </Show>
              </div>
            </div>
          </Card>
        </Show>
      </SettingsPanel>
    );
  }

  return (
    <div class="space-y-6">
      <SettingsPanel
        title="Audit Webhooks"
        description="Configure real-time delivery of security audit events to external systems."
        icon={<Globe class="w-5 h-5" strokeWidth={2} />}
        noPadding
        bodyClass="divide-y divide-border"
      >
        <div class="space-y-6 p-4 sm:p-6">
          <Show when={!canManage()}>
            <div class="rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-200">
              Audit webhook configuration is read-only for this account.
            </div>
          </Show>

          <p class="text-sm text-muted leading-relaxed">
            Pulse can send a signed event payload whenever security-relevant activity occurs
            (logins, settings changes, RBAC updates, and similar audit events).
          </p>

          <Show
            when={!loading()}
            fallback={<p class="text-sm text-muted">Loading audit webhooks…</p>}
          >
            <div class="space-y-3">
              <For each={webhookUrls()}>
                {(url) => (
                  <div class="flex items-center justify-between gap-3 rounded-md border border-border bg-surface-alt p-3">
                    <div class="flex items-center gap-3 overflow-hidden min-w-0">
                      <div class="p-2 bg-blue-100 dark:bg-blue-900 text-blue-600 dark:text-blue-300 rounded-md shrink-0">
                        <ExternalLink size={16} />
                      </div>
                      <span class="text-sm font-medium text-base-content truncate">{url}</span>
                    </div>
                    <button
                      onClick={() => handleRemoveWebhook(url)}
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
                  <p class="text-sm">No audit webhooks configured yet.</p>
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
              Security Note
            </h3>
            <p class="text-sm text-amber-800 dark:text-amber-200 leading-relaxed">
              Audit webhooks are dispatched asynchronously to avoid blocking user operations.
              Endpoints should still verify source trust (for example via an ingest secret) before
              processing events.
            </p>
          </div>
        </div>
      </Card>
    </div>
  );
};
