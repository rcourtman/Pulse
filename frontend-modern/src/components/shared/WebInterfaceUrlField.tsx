import { Component, Show, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { GuestMetadataAPI } from '@/api/guestMetadata';
import { HostMetadataAPI } from '@/api/hostMetadata';

export interface WebInterfaceUrlFieldProps {
  metadataKind: 'guest' | 'host';
  metadataId?: string;
  targetLabel?: string;
  customUrl?: string;
  onCustomUrlChange?: (url: string) => void;
  suggestedUrl?: string;
  suggestedUrlReasonText?: string;
  suggestedUrlReasonTitle?: string;
  suggestedUrlDiagnostic?: string;
  discoveryLoading?: boolean;
  class?: string;
}

const validateCustomUrl = (value: string): string | null => {
  if (!value) return null;
  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    return 'Enter a valid URL (for example: https://192.168.1.100:8080).';
  }

  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    return 'URL must start with http:// or https://.';
  }

  if (!parsed.hostname) {
    return 'URL is missing a hostname or IP address.';
  }

  return null;
};

export const WebInterfaceUrlField: Component<WebInterfaceUrlFieldProps> = (props) => {
  const [urlValue, setUrlValue] = createSignal(props.customUrl ?? '');
  const [fetchedCustomUrl, setFetchedCustomUrl] = createSignal('');
  const [urlSaving, setUrlSaving] = createSignal(false);
  const [urlError, setUrlError] = createSignal<string | null>(null);
  const [urlSuccess, setUrlSuccess] = createSignal<string | null>(null);
  let urlSuccessTimer: ReturnType<typeof setTimeout> | undefined;

  const metadataId = createMemo(() => (props.metadataId || '').trim());
  const targetLabel = createMemo(() =>
    (props.targetLabel || '').trim() || (props.metadataKind === 'host' ? 'host' : 'guest'),
  );
  const currentCustomUrl = createMemo(() => props.customUrl ?? fetchedCustomUrl());
  const normalizedCurrentUrl = createMemo(() => currentCustomUrl().trim());
  const normalizedSuggestedUrl = createMemo(() => (props.suggestedUrl || '').trim());
  const hasSuggestedUrl = createMemo(() => normalizedSuggestedUrl().length > 0);
  const showSuggestedDiagnostic = createMemo(
    () => !props.discoveryLoading && !hasSuggestedUrl() && !!props.suggestedUrlDiagnostic,
  );
  const showSuggestedUrl = createMemo(
    () => hasSuggestedUrl() && normalizedSuggestedUrl() !== normalizedCurrentUrl(),
  );

  const clearUrlSuccessTimer = () => {
    if (!urlSuccessTimer) return;
    clearTimeout(urlSuccessTimer);
    urlSuccessTimer = undefined;
  };

  onCleanup(() => {
    clearUrlSuccessTimer();
  });

  const setUrlSuccessMessage = (message: string) => {
    clearUrlSuccessTimer();
    setUrlSuccess(message);
    urlSuccessTimer = setTimeout(() => {
      setUrlSuccess(null);
      urlSuccessTimer = undefined;
    }, 2500);
  };

  const readMetadataUrl = async (id: string): Promise<string> => {
    if (props.metadataKind === 'host') {
      const metadata = await HostMetadataAPI.getMetadata(id);
      return metadata?.customUrl ?? '';
    }
    const metadata = await GuestMetadataAPI.getMetadata(id);
    return metadata?.customUrl ?? '';
  };

  const updateMetadataUrl = async (id: string, value: string) => {
    if (props.metadataKind === 'host') {
      await HostMetadataAPI.updateMetadata(id, { customUrl: value });
      return;
    }
    await GuestMetadataAPI.updateMetadata(id, { customUrl: value });
  };

  createEffect(() => {
    const id = metadataId();
    if (!id || props.customUrl !== undefined) return;

    let cancelled = false;
    const loadMetadata = async () => {
      try {
        const customUrl = await readMetadataUrl(id);
        if (!cancelled) {
          setFetchedCustomUrl(customUrl);
        }
      } catch {
        // Metadata fetch is best-effort; keep the field editable even if load fails.
      }
    };

    void loadMetadata();

    onCleanup(() => {
      cancelled = true;
    });
  });

  createEffect(() => {
    setUrlValue(currentCustomUrl());
  });

  const handleSaveUrl = async () => {
    const id = metadataId();
    if (!id) return;
    const trimmed = urlValue().trim();
    setUrlError(null);
    setUrlSuccess(null);

    const validationError = validateCustomUrl(trimmed);
    if (validationError) {
      setUrlError(validationError);
      return;
    }

    setUrlSaving(true);
    try {
      await updateMetadataUrl(id, trimmed);
      setFetchedCustomUrl(trimmed);
      props.onCustomUrlChange?.(trimmed);
      setUrlSuccessMessage(trimmed ? 'URL saved.' : 'URL cleared.');
    } catch (err) {
      setUrlError(err instanceof Error ? err.message : 'Failed to save URL.');
      console.error('Failed to save custom URL:', err);
    } finally {
      setUrlSaving(false);
    }
  };

  const handleDeleteUrl = async () => {
    const id = metadataId();
    if (!id) return;
    setUrlError(null);
    setUrlSuccess(null);
    setUrlSaving(true);
    try {
      await updateMetadataUrl(id, '');
      setFetchedCustomUrl('');
      setUrlValue('');
      props.onCustomUrlChange?.('');
      setUrlSuccessMessage('URL removed.');
    } catch (err) {
      setUrlError(err instanceof Error ? err.message : 'Failed to remove URL.');
      console.error('Failed to remove custom URL:', err);
    } finally {
      setUrlSaving(false);
    }
  };

  return (
    <Show when={metadataId()}>
      <div class={`rounded border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-600 dark:bg-slate-800 ${props.class ?? ''}`}>
        <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">
          Web Interface URL
        </div>
        <div class="flex items-center gap-2">
          <input
            type="url"
            class="flex-1 text-xs px-2.5 py-1.5 border border-slate-300 dark:border-slate-600 rounded-md bg-surface text-base-content focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-colors"
            placeholder="https://192.168.1.100:8080"
            value={urlValue()}
            onInput={(e) => setUrlValue(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                void handleSaveUrl();
              }
            }}
            disabled={urlSaving()}
          />
          <button
            type="button"
            class="px-2.5 py-1.5 text-xs font-medium rounded-md bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 transition-colors"
            disabled={urlSaving() || urlValue().trim() === normalizedCurrentUrl()}
            onClick={() => void handleSaveUrl()}
          >
            Save
          </button>
          <Show when={normalizedCurrentUrl()}>
            <a
              href={normalizedCurrentUrl()}
              target="_blank"
              rel="noopener noreferrer"
              class="px-2.5 py-1.5 text-xs font-medium rounded-md text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-900 transition-colors"
              title="Open URL"
            >
              Open
            </a>
          </Show>
          <Show when={normalizedCurrentUrl()}>
            <button
              type="button"
              class="px-2.5 py-1.5 text-xs font-medium rounded-md text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900 disabled:opacity-50 transition-colors"
              disabled={urlSaving()}
              onClick={() => void handleDeleteUrl()}
              title="Remove URL"
            >
              Remove
            </button>
          </Show>
        </div>

        <Show when={urlError()}>
          <p class="mt-1.5 text-[11px] text-red-600 dark:text-red-400">{urlError()}</p>
        </Show>
        <Show when={urlSuccess()}>
          <p class="mt-1.5 text-[11px] text-emerald-600 dark:text-emerald-400">{urlSuccess()}</p>
        </Show>

        <Show when={showSuggestedDiagnostic()}>
          <div class="mt-2 rounded border border-amber-200 bg-amber-50 p-2 text-[11px] text-amber-800 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-200">
            <p class="font-medium">No suggested URL found</p>
            <p class="mt-0.5">{props.suggestedUrlDiagnostic}</p>
          </div>
        </Show>

        <Show when={showSuggestedUrl()}>
          <div class="mt-2 p-2 rounded bg-blue-50 border border-blue-200 dark:bg-blue-900 dark:border-blue-800">
            <div class="text-[10px] font-medium text-blue-700 dark:text-blue-300 mb-1">
              {normalizedCurrentUrl() ? 'Discovered URL' : 'Suggested URL'}
            </div>
            <Show when={props.suggestedUrlReasonText}>
              <p
                class="mb-1 text-[10px] text-blue-700 dark:text-blue-300"
                title={props.suggestedUrlReasonTitle}
              >
                Why this URL: {props.suggestedUrlReasonText}
              </p>
            </Show>
            <div class="flex items-center gap-2">
              <code
                class="flex-1 text-xs text-blue-800 dark:text-blue-200 font-mono truncate"
                title={normalizedSuggestedUrl()}
              >
                {normalizedSuggestedUrl()}
              </code>
              <button
                type="button"
                class="px-2 py-1 text-xs font-medium rounded bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 transition-colors flex-shrink-0"
                onClick={() => setUrlValue(normalizedSuggestedUrl())}
                disabled={urlSaving()}
              >
                {normalizedCurrentUrl() ? 'Use instead' : 'Use this'}
              </button>
            </div>
          </div>
        </Show>

        <p class="mt-1.5 text-[10px] text-muted">
          Add a URL to quickly access this {targetLabel()}'s web interface from the dashboard.
        </p>
      </div>
    </Show>
  );
};

export default WebInterfaceUrlField;
