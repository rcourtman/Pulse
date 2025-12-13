import { Component, createSignal, Show, onMount } from 'solid-js';
import { Portal } from 'solid-js/web';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { isPulseHttps } from '@/utils/url';
import { logger } from '@/utils/logger';

interface SecurityStatus {
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
  const [status, setStatus] = createSignal<SecurityStatus | null>(null);
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
      const response = await fetch('/api/security/status');
      if (response.ok) {
        const data = await response.json();

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
      }
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

  const getScoreColor = (score: number, max: number) => {
    const percentage = (score / max) * 100;
    if (percentage >= 80) return 'text-green-600 dark:text-green-400';
    if (percentage >= 60) return 'text-yellow-600 dark:text-yellow-400';
    if (percentage >= 40) return 'text-orange-600 dark:text-orange-400';
    return 'text-red-600 dark:text-red-400';
  };

  const getScoreIcon = (score: number, max: number) => {
    const percentage = (score / max) * 100;
    if (percentage >= 80) return 'shield';
    if (percentage >= 60) return 'warning';
    return 'alert';
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

  return (
    <Portal>
      <div
        class={`fixed top-0 left-0 right-0 z-50 border-b shadow-sm ${status()!.publicAccess && !status()!.hasAuthentication
            ? 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800'
            : 'bg-yellow-50 dark:bg-yellow-900/20 border-yellow-200 dark:border-yellow-800'
          }`}
      >
        <div class="max-w-7xl mx-auto px-4 py-3">
          <div class="flex items-start justify-between">
            <div class="flex items-start space-x-3">
              <span class={`text-2xl ${getScoreIcon(status()!.score, status()!.maxScore) === 'shield' ? 'text-green-600' : getScoreIcon(status()!.score, status()!.maxScore) === 'warning' ? 'text-yellow-600' : 'text-red-600'}`}>
                {getScoreIcon(status()!.score, status()!.maxScore) === 'shield' ? '✓' : getScoreIcon(status()!.score, status()!.maxScore) === 'warning' ? '!' : '!!'}
              </span>
              <div>
                <div class="flex items-center gap-3">
                  <SectionHeader
                    title={
                      <span>
                        Security score:{' '}
                        <span class={getScoreColor(status()!.score, status()!.maxScore)}>
                          {status()!.score}/{status()!.maxScore}
                        </span>
                      </span>
                    }
                    size="sm"
                    class="flex-1"
                    titleClass="text-gray-900 dark:text-gray-100"
                  />
                  <button
                    type="button"
                    onClick={() => setShowDetails(!showDetails())}
                    class="text-sm text-blue-600 dark:text-blue-400 hover:underline"
                  >
                    {showDetails() ? 'Hide' : 'Show'} Details
                  </button>
                </div>

                <p class="text-sm text-gray-700 dark:text-gray-300 mt-1">
                  {status()!.publicAccess ? (
                    <span class="font-semibold text-red-700 dark:text-red-300">
                      WARNING: PUBLIC NETWORK ACCESS DETECTED - Your Proxmox credentials are exposed to
                      the internet!
                    </span>
                  ) : (
                    'Your Pulse instance is accessible without authentication. Proxmox credentials could be exposed.'
                  )}
                </p>

                <Show when={showDetails()}>
                  <div class="mt-3 space-y-1">
                    <div class="text-xs space-y-1">
                      <div class="flex items-center gap-2">
                        <span
                          class={status()!.credentialsEncrypted ? 'text-green-600' : 'text-red-600'}
                        >
                          {status()!.credentialsEncrypted ? 'Yes' : 'No'}
                        </span>
                        <span>Credentials encrypted at rest</span>
                      </div>
                      <div class="flex items-center gap-2">
                        <span class={status()!.exportProtected ? 'text-green-600' : 'text-red-600'}>
                          {status()!.exportProtected ? 'Yes' : 'No'}
                        </span>
                        <span>Export requires authentication</span>
                      </div>
                      <div class="flex items-center gap-2">
                        <span
                          class={status()!.hasAuthentication ? 'text-green-600' : 'text-red-600'}
                        >
                          {status()!.hasAuthentication ? 'Yes' : 'No'}
                        </span>
                        <span>Authentication enabled</span>
                      </div>
                      <div class="flex items-center gap-2">
                        <span class={status()!.hasHTTPS ? 'text-green-600' : 'text-red-600'}>
                          {status()!.hasHTTPS ? 'Yes' : 'No'}
                        </span>
                        <span>HTTPS connection</span>
                      </div>
                      <div class="flex items-center gap-2">
                        <span class={status()!.hasAuditLogging ? 'text-green-600' : 'text-red-600'}>
                          {status()!.hasAuditLogging ? 'Yes' : 'No'}
                        </span>
                        <span>Audit logging enabled</span>
                      </div>
                    </div>
                  </div>
                </Show>

                <div class="flex items-center gap-3 mt-3">
                  <a
                    href="/settings?tab=security"
                    class="text-sm font-medium text-blue-600 dark:text-blue-400 hover:underline"
                  >
                    Enable Security →
                  </a>
                  <a
                    href="https://github.com/rcourtman/Pulse/blob/main/docs/SECURITY.md"
                    target="_blank"
                    class="text-sm text-gray-600 dark:text-gray-400 hover:underline"
                  >
                    Learn More
                  </a>
                  <div class="relative group">
                    <button
                      type="button"
                      onClick={() => handleDismiss('day')}
                      class="text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300"
                    >
                      Dismiss ▼
                    </button>
                    <div class="absolute left-0 top-full mt-1 bg-white dark:bg-gray-800 rounded shadow-lg border border-gray-200 dark:border-gray-700 opacity-0 group-hover:opacity-100 pointer-events-none group-hover:pointer-events-auto transition-opacity">
                      <button
                        type="button"
                        onClick={() => handleDismiss('day')}
                        class="block w-full text-left px-3 py-1.5 text-sm hover:bg-gray-100 dark:hover:bg-gray-700"
                      >
                        For 1 day
                      </button>
                      <button
                        type="button"
                        onClick={() => handleDismiss('week')}
                        class="block w-full text-left px-3 py-1.5 text-sm hover:bg-gray-100 dark:hover:bg-gray-700"
                      >
                        For 1 week
                      </button>
                      <button
                        type="button"
                        onClick={() => handleDismiss('forever')}
                        class="block w-full text-left px-3 py-1.5 text-sm hover:bg-gray-100 dark:hover:bg-gray-700"
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
