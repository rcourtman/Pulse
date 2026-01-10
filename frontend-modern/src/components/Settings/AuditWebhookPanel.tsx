import { createSignal, For, onMount, Show, createEffect } from 'solid-js';
import Shield from 'lucide-solid/icons/shield';
import Globe from 'lucide-solid/icons/globe';
import Plus from 'lucide-solid/icons/plus';
import Trash2 from 'lucide-solid/icons/trash-2';
import ExternalLink from 'lucide-solid/icons/external-link';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
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

    return (
        <div class="space-y-6">
            <SectionHeader
                title={<>Audit Webhooks</>}
                description={<>Configure real-time delivery of audit events to external systems.</>}
            />

            <Card>
                <div class="p-6 space-y-6">
                    <p class="text-slate-400 text-sm leading-relaxed">
                        Whenever a security-relevant event occurs (login, config change, RBAC update),
                        Pulse can send a POST request with the JSON-encoded event data to the following endpoints.
                    </p>

                    <div class="space-y-4">
                        <For each={webhookUrls()}>
                            {(url) => (
                                <div class="flex items-center justify-between p-3 bg-slate-800/50 border border-slate-700 rounded-lg group">
                                    <div class="flex items-center gap-3 overflow-hidden">
                                        <div class="p-2 bg-blue-500/10 text-blue-400 rounded-md shrink-0">
                                            <ExternalLink size={16} />
                                        </div>
                                        <span class="text-sm font-medium text-slate-200 truncate">{url}</span>
                                    </div>
                                    <button
                                        onClick={() => handleRemoveWebhook(url)}
                                        class="p-2 text-slate-500 hover:text-red-400 hover:bg-red-400/10 rounded-md transition-colors"
                                        title="Remove Webhook"
                                    >
                                        <Trash2 size={16} />
                                    </button>
                                </div>
                            )}
                        </For>

                        <Show when={webhookUrls().length === 0 && !loading()}>
                            <div class="py-12 flex flex-col items-center justify-center text-slate-500 border-2 border-dashed border-slate-800 rounded-xl">
                                <Globe size={48} class="opacity-20 mb-4" />
                                <p>No audit webhooks configured yet.</p>
                            </div>
                        </Show>
                    </div>

                    <div class="flex gap-3 pt-4 border-t border-slate-800">
                        <input
                            type="text"
                            placeholder="https://your-api.com/webhook"
                            class="flex-1 bg-slate-900 border border-slate-700 rounded-lg px-4 py-2 text-slate-200 focus:outline-none focus:border-blue-500 transition-colors"
                            value={newUrl()}
                            onInput={(e) => setNewUrl(e.currentTarget.value)}
                            onKeyDown={(e) => e.key === 'Enter' && handleAddWebhook()}
                        />
                        <button
                            onClick={handleAddWebhook}
                            disabled={saving() || !newUrl()}
                            class="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:bg-slate-700 text-white rounded-lg flex items-center gap-2 transition-all shadow-lg shadow-blue-900/20"
                        >
                            <Plus size={18} />
                            Add Endpoint
                        </button>
                    </div>
                </div>
            </Card>

            <div class="bg-amber-900/10 border border-amber-900/20 rounded-xl p-6">
                <div class="flex gap-4">
                    <div class="p-3 bg-amber-600/20 rounded-lg h-fit text-amber-500">
                        <Shield size={24} />
                    </div>
                    <div>
                        <h3 class="text-lg font-bold text-white mb-2">Security Note</h3>
                        <p class="text-slate-400 text-sm leading-relaxed">
                            Webhooks are dispatched asynchronously to ensure zero latency impact on operations.
                            Each request includes the tamper-proof event payload, but endpoints should still
                            verify source IP or implement an ingest secret for maximum security.
                        </p>
                    </div>
                </div>
            </div>
        </div>
    );
}
