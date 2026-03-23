import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { GuestMetadataAPI } from '@/api/guestMetadata';
import { AgentMetadataAPI } from '@/api/agentMetadata';
import {
  getWebInterfaceSuggestedUrlFallback,
  getWebInterfaceTargetLabel,
  normalizeWebInterfaceUrl,
  shouldShowWebInterfaceSuggestedDiagnostic,
  shouldShowWebInterfaceSuggestedUrl,
  validateWebInterfaceCustomUrl,
  type WebInterfaceUrlFieldProps,
} from './webInterfaceUrlFieldModel';

export function useWebInterfaceUrlFieldState(props: WebInterfaceUrlFieldProps) {
  const [urlValue, setUrlValue] = createSignal(props.customUrl ?? '');
  const [fetchedCustomUrl, setFetchedCustomUrl] = createSignal('');
  const [urlSaving, setUrlSaving] = createSignal(false);
  const [urlError, setUrlError] = createSignal<string | null>(null);
  const [urlSuccess, setUrlSuccess] = createSignal<string | null>(null);
  let urlSuccessTimer: ReturnType<typeof setTimeout> | undefined;

  const metadataId = createMemo(() => normalizeWebInterfaceUrl(props.metadataId));
  const targetLabel = createMemo(() =>
    getWebInterfaceTargetLabel(props.metadataKind, props.targetLabel),
  );
  const currentCustomUrl = createMemo(() => props.customUrl ?? fetchedCustomUrl());
  const normalizedCurrentUrl = createMemo(() => normalizeWebInterfaceUrl(currentCustomUrl()));
  const normalizedSuggestedUrl = createMemo(() => normalizeWebInterfaceUrl(props.suggestedUrl));
  const showSuggestedDiagnostic = createMemo(() =>
    shouldShowWebInterfaceSuggestedDiagnostic({
      discoveryLoading: props.discoveryLoading,
      suggestedUrl: props.suggestedUrl,
      suggestedUrlDiagnostic: props.suggestedUrlDiagnostic,
    }),
  );
  const showSuggestedUrl = createMemo(() =>
    shouldShowWebInterfaceSuggestedUrl({
      currentUrl: currentCustomUrl(),
      suggestedUrl: props.suggestedUrl,
    }),
  );
  const suggestedUrlFallback = createMemo(() =>
    getWebInterfaceSuggestedUrlFallback(props.suggestedUrlDiagnostic),
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
    if (props.metadataKind === 'agent') {
      const metadata = await AgentMetadataAPI.getMetadata(id);
      return metadata?.customUrl ?? '';
    }
    const metadata = await GuestMetadataAPI.getMetadata(id);
    return metadata?.customUrl ?? '';
  };

  const updateMetadataUrl = async (id: string, value: string) => {
    if (props.metadataKind === 'agent') {
      await AgentMetadataAPI.updateMetadata(id, { customUrl: value });
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

    const trimmed = normalizeWebInterfaceUrl(urlValue());
    setUrlError(null);
    setUrlSuccess(null);

    const validationError = validateWebInterfaceCustomUrl(trimmed);
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
    } catch (error) {
      setUrlError(error instanceof Error ? error.message : 'Failed to save URL.');
      console.error('Failed to save custom URL:', error);
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
    } catch (error) {
      setUrlError(error instanceof Error ? error.message : 'Failed to remove URL.');
      console.error('Failed to remove custom URL:', error);
    } finally {
      setUrlSaving(false);
    }
  };

  return {
    currentCustomUrl,
    handleDeleteUrl,
    handleSaveUrl,
    metadataId,
    normalizedCurrentUrl,
    normalizedSuggestedUrl,
    setUrlValue,
    showSuggestedDiagnostic,
    showSuggestedUrl,
    suggestedUrlFallback,
    targetLabel,
    urlError,
    urlSaving,
    urlSuccess,
    urlValue,
  };
}

export type WebInterfaceUrlFieldState = ReturnType<typeof useWebInterfaceUrlFieldState>;
