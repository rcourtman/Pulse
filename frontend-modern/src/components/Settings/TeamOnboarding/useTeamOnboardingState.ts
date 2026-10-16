import { createSignal, onCleanup, onMount } from 'solid-js';
import { SecurityAPI } from '@/api/security';
import { getTeamSignInMethods, getTeamSignInUrl } from '@/utils/rbacPresentation';

export function useTeamOnboardingState() {
  const [loading, setLoading] = createSignal(true);
  const [loadError, setLoadError] = createSignal(false);
  const [signInMethods, setSignInMethods] = createSignal<string[]>([]);
  const [copied, setCopied] = createSignal(false);
  const [copyError, setCopyError] = createSignal(false);
  const [copying, setCopying] = createSignal(false);
  const signInUrl = getTeamSignInUrl(window.location.origin);
  let active = true;
  let request = 0;

  onCleanup(() => {
    active = false;
  });

  const load = async () => {
    const currentRequest = ++request;
    setLoading(true);
    setLoadError(false);
    setSignInMethods([]);
    try {
      const status = await SecurityAPI.getStatus();
      if (active && currentRequest === request) setSignInMethods(getTeamSignInMethods(status));
    } catch {
      if (active && currentRequest === request) setLoadError(true);
    } finally {
      if (active && currentRequest === request) setLoading(false);
    }
  };

  const copyLink = async () => {
    if (!signInUrl || loading() || loadError() || !signInMethods().length || copying()) return;
    setCopying(true);
    setCopied(false);
    setCopyError(false);
    try {
      await navigator.clipboard.writeText(signInUrl);
      if (active) setCopied(true);
    } catch {
      if (active) setCopyError(true);
    } finally {
      if (active) setCopying(false);
    }
  };

  onMount(() => {
    void load();
  });

  return {
    loading,
    loadError,
    signInMethods,
    signInUrl,
    copied,
    copyError,
    copying,
    load,
    copyLink,
  };
}
