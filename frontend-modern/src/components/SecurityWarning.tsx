import { Component, createSignal, Show, onMount } from 'solid-js';
import { Portal } from 'solid-js/web';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { isPulseHttps } from '@/utils/url';
import { logger } from '@/utils/logger';
import { apiFetchJSON } from '@/utils/apiClient';
import {
  getSecurityFeatureStatePresentation,
  getSecurityScorePresentation,
  getSecurityScoreSymbol,
  getSecurityScoreTextClass,
  getSecurityWarningPresentation,
} from '@/utils/securityScorePresentation';

import type { SecurityStatus } from '@/types/config';

interface SecurityState {
  hasAuthentication: boolean;
  hasHTTPS: boolean;
  hasAPIToken: boolean;
  hasAuditLogging: boolean;
  credentialsEncrypted: boolean;
  exportProtected: boolean;
  score: number;
  maxScore: number;
  publicAccess?: boolean;
  isPrivateNetwork?: boolean;
  clientIP?: string;
}

export const SecurityWarning: Component = () => {
  const [dismissed, setDismissed] = createSignal(false);
  const [status, setStatus] = createSignal<SecurityState | null>(null);
  const [showDetails, setShowDetails] = createSignal(false);

  onMount(async () => {
    // Check if user has previously dismissed
    const dismissedUntil = localStorage.getItem('securityWarningDismissed');
    if (dismissedUntil) {
      const dismissDate = new Date(dismissedUntil);
      if (dismissDate > new Date()) {
        setDismissed(true);
        return;
      }
    }

    // Fetch security status
    try {
      const data = await apiFetchJSON<SecurityStatus>('/api/security/status');

      // Calculate security score
      let score = 0;
      const maxScore = 5;

      const runningOverHttps = isPulseHttps();

      if (data.credentialsEncrypted !== false) score++; // Always true currently
      if (data.exportProtected) score++;
      if (data.apiTokenConfigured) score++;
      if (data.hasHTTPS || runningOverHttps) score++;
      if (data.hasAuthentication) score++;

      setStatus({
        hasAuthentication: data.hasAuthentication || false,
        hasHTTPS: runningOverHttps,
        hasAPIToken: data.apiTokenConfigured || false,
        hasAuditLogging: data.hasAuditLogging || false,
        credentialsEncrypted: true, // Always true in current implementation
        exportProtected: data.exportProtected || false,
        score,
        maxScore,
        publicAccess: data.publicAccess || false,
        isPrivateNetwork: data.isPrivateNetwork,
        clientIP: data.clientIP,
      });
    } catch (error) {
      logger.error('Failed to fetch security status:', error);
    }
  });

  const handleDismiss = (duration: 'day' | 'week' | 'forever') => {
    const now = new Date();
    if (duration === 'day') {
      now.setDate(now.getDate() + 1);
    } else if (duration === 'week') {
      now.setDate(now.getDate() + 7);
    } else {
      now.setFullYear(now.getFullYear() + 100); // "Forever"
    }
    localStorage.setItem('securityWarningDismissed', now.toISOString());
    setDismissed(true);
  };

  // Show more aggressively if public access detected
  const shouldShow = () => {
    if (dismissed()) return false;
    if (!status()) return false;

    // Always show if public access without auth
    if (status()!.publicAccess && !status()!.hasAuthentication) {
      return true;
    }

    // Show if score is low
    return status()!.score < 4;
  };

  if (!shouldShow()) {
    return null;
  }

  const scorePercentage = () => (status()!.score / status()!.maxScore) * 100;
  const scorePresentation = () => getSecurityScorePresentation(scorePercentage());
  const warningPresentation = () =>
    getSecurityWarningPresentation({
      score: scorePercentage(),
      publicAccess: status()!.publicAccess || false,
      hasAuthentication: status()!.hasAuthentication,
    });

  return (
    <Portal>
      <div
        class={`fixed top-0 left-0 right-0 z-50 border-b shadow-sm ${warningPresentation().background} ${warningPresentation().border}`}
      >
        <div class="max-w-7xl mx-auto px-4 py-3">
          <div class="flex items-start justify-between">
            <div class="flex items-start space-x-3">
              <span class={`text-2xl ${scorePresentation().tone.icon}`}>
                {getSecurityScoreSymbol(scorePercentage())}
              </span>
              <div>
                <div class="flex items-center gap-3">
                  <SectionHeader
                    title={
                      <span>
                        Security score:{' '}
                        <span class={getSecurityScoreTextClass(scorePercentage())}>
                          {status()!.score}/{status()!.maxScore}
                        </span>
                      </span>
                    }
                    size="sm"
                    class="flex-1"
                    titleClass="text-base-content"
                  />
                  <button
                    type="button"
                    onClick={() => setShowDetails(!showDetails())}
                    class="text-sm text-blue-600 dark:text-blue-400 hover:underline"
                  >
                    {showDetails() ? 'Hide' : 'Show'} Details
                  </button>
                </div>

                <p class="text-sm text-base-content mt-1">
                  <span class={warningPresentation().messageClass}>
                    {warningPresentation().message}
                  </span>
                </p>

                <Show when={showDetails()}>
                  <div class="mt-3 space-y-1">
                    <div class="text-xs space-y-1">
                      <div class="flex items-center gap-2">
                        <span class={getSecurityFeatureStatePresentation(status()!.credentialsEncrypted).className}>
                          {getSecurityFeatureStatePresentation(status()!.credentialsEncrypted).label}
                        </span>
                        <span>Credentials encrypted at rest</span>
                      </div>
                      <div class="flex items-center gap-2">
                        <span class={getSecurityFeatureStatePresentation(status()!.exportProtected).className}>
                          {getSecurityFeatureStatePresentation(status()!.exportProtected).label}
                        </span>
                        <span>Export requires authentication</span>
                      </div>
                      <div class="flex items-center gap-2">
                        <span class={getSecurityFeatureStatePresentation(status()!.hasAuthentication).className}>
                          {getSecurityFeatureStatePresentation(status()!.hasAuthentication).label}
                        </span>
                        <span>Authentication enabled</span>
                      </div>
                      <div class="flex items-center gap-2">
                        <span class={getSecurityFeatureStatePresentation(status()!.hasHTTPS).className}>
                          {getSecurityFeatureStatePresentation(status()!.hasHTTPS).label}
                        </span>
                        <span>HTTPS connection</span>
                      </div>
                      <div class="flex items-center gap-2">
                        <span class={getSecurityFeatureStatePresentation(status()!.hasAuditLogging).className}>
                          {getSecurityFeatureStatePresentation(status()!.hasAuditLogging).label}
                        </span>
                        <span>Audit logging enabled</span>
                      </div>
                    </div>
                  </div>
                </Show>

                <div class="flex items-center gap-3 mt-3">
                  <a
                    href="/settings/security-overview"
                    class="text-sm font-medium text-blue-600 dark:text-blue-400 hover:underline"
                  >
                    Enable Security →
                  </a>
                  <a
                    href="https://github.com/rcourtman/Pulse/blob/main/docs/SECURITY.md"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="text-sm text-muted hover:underline"
                  >
                    Learn More
                  </a>
                  <div class="relative group">
                    <button
                      type="button"
                      onClick={() => handleDismiss('day')}
                      class="text-sm text-muted hover:text-base-content"
                    >
                      Dismiss ▼
                    </button>
                    <div class="absolute left-0 top-full mt-1 bg-surface rounded shadow-sm border border-border opacity-0 group-hover:opacity-100 pointer-events-none group-hover:pointer-events-auto transition-opacity">
                      <button
                        type="button"
                        onClick={() => handleDismiss('day')}
                        class="block w-full text-left px-3 py-1.5 text-sm hover:bg-surface-hover"
                      >
                        For 1 day
                      </button>
                      <button
                        type="button"
                        onClick={() => handleDismiss('week')}
                        class="block w-full text-left px-3 py-1.5 text-sm hover:bg-surface-hover"
                      >
                        For 1 week
                      </button>
                      <button
                        type="button"
                        onClick={() => handleDismiss('forever')}
                        class="block w-full text-left px-3 py-1.5 text-sm hover:bg-surface-hover"
                      >
                        Forever
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Portal>
  );
};
