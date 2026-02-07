import { createSignal, For, onMount, Show, createEffect } from 'solid-js';
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
import { hasFeature, loadLicenseStatus } from '@/stores/license';

export function AuditWebhookPanel() {
    const [webhookUrls, setWebhookUrls] = createSignal<string[]>([]);
    const [newUrl, setNewUrl] = createSignal('');
    const [saving, setSaving] = createSignal(false);
    const [loading, setLoading] = createSignal(true);

    onMount(() => {
        loadLicenseStatus();
    });

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
        const updated = webhookUrls().filter(u => u !== url);
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
                <Show when={!loading()} fallback={<div class="text-sm text-gray-500 dark:text-gray-400">Loading...</div>}>
                    <Card tone="info" padding="md">
                        <div class="text-sm">
                            <p class="font-semibold text-gray-900 dark:text-gray-100">Pulse Pro Required</p>
                            <p class="mt-1 text-gray-600 dark:text-gray-400">
                                Audit webhooks are part of the audit logging feature set and require Pulse Pro.
                            </p>
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
                bodyClass="space-y-6"
            >
                <p class="text-sm text-gray-600 dark:text-gray-400 leading-relaxed">
                    Pulse can send a signed event payload whenever security-relevant activity occurs
                    (logins, settings changes, RBAC updates, and similar audit events).
                </p>

                <Show when={!loading()} fallback={<p class="text-sm text-gray-500 dark:text-gray-400">Loading audit webhooks…</p>}>
                    <div class="space-y-3">
                        <For each={webhookUrls()}>
                            {(url) => (
                                <div class="flex items-center justify-between gap-3 rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/40 p-3">
                                    <div class="flex items-center gap-3 overflow-hidden min-w-0">
                                        <div class="p-2 bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-300 rounded-md shrink-0">
                                            <ExternalLink size={16} />
                                        </div>
                                        <span class="text-sm font-medium text-gray-800 dark:text-gray-200 truncate">{url}</span>
                                    </div>
                                    <button
                                        onClick={() => handleRemoveWebhook(url)}
                                        class="p-2 text-gray-500 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/30 rounded-md transition-colors"
                                        title="Remove webhook endpoint"
                                    >
                                        <Trash2 size={16} />
                                    </button>
                                </div>
                            )}
                        </For>

                        <Show when={webhookUrls().length === 0}>
                            <div class="py-10 flex flex-col items-center justify-center text-gray-500 dark:text-gray-400 border-2 border-dashed border-gray-300 dark:border-gray-700 rounded-xl">
                                <Globe size={36} class="opacity-40 mb-3" />
                                <p class="text-sm">No audit webhooks configured yet.</p>
                            </div>
                        </Show>
                    </div>
                </Show>

                <div class="flex gap-3 pt-4 border-t border-gray-200 dark:border-gray-700">
                    <input
                        type="text"
                        placeholder="https://your-api.com/webhook"
                        class={`${formControl} flex-1`}
                        value={newUrl()}
                        onInput={(e) => setNewUrl(e.currentTarget.value)}
                        onKeyDown={(e) => e.key === 'Enter' && handleAddWebhook()}
                    />
                    <button
                        onClick={handleAddWebhook}
                        disabled={saving() || !newUrl().trim()}
                        class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg flex items-center gap-2 transition-colors"
                    >
                        <Plus size={18} />
                        Add Endpoint
                    </button>
                </div>
            </SettingsPanel>

            <Card tone="warning" class="border border-amber-200 dark:border-amber-800">
                <div class="p-5 flex gap-4">
                    <div class="p-3 bg-amber-100 dark:bg-amber-900/40 rounded-lg h-fit text-amber-600 dark:text-amber-300">
                        <Shield size={22} />
                    </div>
                    <div>
                        <h3 class="text-base font-semibold text-amber-900 dark:text-amber-100 mb-1.5">Security Note</h3>
                        <p class="text-sm text-amber-800 dark:text-amber-200 leading-relaxed">
                            Audit webhooks are dispatched asynchronously to avoid blocking user operations.
                            Endpoints should still verify source trust (for example via an ingest secret) before processing events.
                        </p>
                    </div>
                </div>
            </Card>
        </div>
    );
}
