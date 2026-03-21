import { Component, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import type { NetworkBoundarySettingsSectionProps } from './networkSettingsModel';

interface EnvironmentOverrideAlertProps {
  description: string;
  title: string;
}

function EnvironmentOverrideAlert(props: EnvironmentOverrideAlertProps) {
  return (
    <div class="mt-2 p-2 bg-amber-100 dark:bg-amber-900 border border-amber-300 dark:border-amber-700 rounded text-xs text-amber-800 dark:text-amber-200">
      <div class="flex items-center gap-1">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
          />
        </svg>
        <span>{props.title}</span>
      </div>
      <div class="mt-1 text-amber-700 dark:text-amber-300">{props.description}</div>
    </div>
  );
}

export const NetworkBoundarySettingsSection: Component<NetworkBoundarySettingsSectionProps> = (
  props,
) => {
  return (
    <>
      <section class="p-4 sm:p-6 space-y-4">
        <h4 class="flex items-center gap-2 text-sm font-medium text-base-content">
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <path d="M10 13a5 5 0 007.54.54l3-3a5 5 0 00-7.07-7.07l-1.72 1.71"></path>
            <path d="M14 11a5 5 0 00-7.54-.54l-3 3a5 5 0 007.07 7.07l1.71-1.71"></path>
          </svg>
          Public URL
        </h4>
        <div class="space-y-2">
          <label class="text-sm font-medium text-base-content">Dashboard URL for Notifications</label>
          <p class="text-xs text-muted">
            The URL included in email alerts to link back to Pulse. Required for Docker deployments
            with custom ports.
          </p>
          <div class="relative">
            <input
              type="text"
              value={props.publicURL()}
              onInput={(e) => {
                if (!props.envOverrides().publicURL) {
                  props.setPublicURL(e.currentTarget.value);
                  props.setHasUnsavedChanges(true);
                }
              }}
              disabled={props.envOverrides().publicURL}
              placeholder="http://192.168.1.100:8080"
              class={`w-full min-h-10 sm:min-h-10 px-3 py-2.5 text-sm border rounded-md ${
                props.envOverrides().publicURL
                  ? 'border-amber-300 dark:border-amber-600 bg-amber-50 dark:bg-amber-900 cursor-not-allowed opacity-75'
                  : 'border-border bg-surface'
              }`}
            />
            <Show when={props.envOverrides().publicURL}>
              <EnvironmentOverrideAlert
                title="Overridden by PULSE_PUBLIC_URL environment variable"
                description="Remove the env var and restart to enable UI configuration"
              />
            </Show>
          </div>
          <p class="text-xs text-muted mt-1">
            Example: If you access Pulse at <code>http://myserver:8080</code>, enter that URL here.
            Leave empty to auto-detect (may not work correctly with Docker port mappings).
          </p>
        </div>
      </section>

      <section class="p-4 sm:p-6 space-y-4">
        <h4 class="flex items-center gap-2 text-sm font-medium text-base-content">
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <circle cx="12" cy="12" r="10"></circle>
            <path d="M2 12h20M12 2a15.3 15.3 0 014 10 15.3 15.3 0 01-4 10 15.3 15.3 0 01-4-10 15.3 15.3 0 014-10z"></path>
          </svg>
          Network Settings
        </h4>
        <div class="space-y-2">
          <label class="text-sm font-medium text-base-content">CORS Allowed Origins</label>
          <p class="text-xs text-muted">
            For reverse proxy setups (* = allow all, empty = same-origin only)
          </p>
          <div class="relative">
            <input
              type="text"
              value={props.allowedOrigins()}
              onInput={(e) => {
                if (!props.envOverrides().allowedOrigins) {
                  props.setAllowedOrigins(e.currentTarget.value);
                  props.setHasUnsavedChanges(true);
                }
              }}
              disabled={props.envOverrides().allowedOrigins}
              placeholder="* or https://example.com"
              class={`w-full min-h-10 sm:min-h-10 px-3 py-2.5 text-sm border rounded-md ${
                props.envOverrides().allowedOrigins
                  ? 'border-amber-300 dark:border-amber-600 bg-amber-50 dark:bg-amber-900 cursor-not-allowed opacity-75'
                  : 'border-border bg-surface'
              }`}
            />
            <Show when={props.envOverrides().allowedOrigins}>
              <EnvironmentOverrideAlert
                title="Overridden by ALLOWED_ORIGINS environment variable"
                description="Remove the env var and restart to enable UI configuration"
              />
            </Show>
          </div>
        </div>
      </section>

      <section class="p-4 sm:p-6 space-y-4">
        <h4 class="flex items-center gap-2 text-sm font-medium text-base-content">
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <rect x="3" y="4" width="18" height="14" rx="2"></rect>
            <path d="M7 20h10"></path>
          </svg>
          Embedding
        </h4>
        <p class="text-xs text-muted">
          Allow Pulse to be embedded in iframes (e.g., Homepage dashboard)
        </p>
        <div class="space-y-3">
          <div class="flex items-center gap-2">
            <input
              type="checkbox"
              id="allowEmbedding"
              checked={props.allowEmbedding()}
              onChange={(e) => {
                props.setAllowEmbedding(e.currentTarget.checked);
                props.setHasUnsavedChanges(true);
              }}
              class="h-5 w-5 sm:h-4 sm:w-4 rounded border-border text-blue-600 focus:ring-blue-500"
            />
            <label for="allowEmbedding" class="text-sm text-base-content">
              Allow iframe embedding
            </label>
          </div>

          <Show when={props.allowEmbedding()}>
            <div class="space-y-2">
              <label class="text-xs font-medium text-base-content">
                Allowed Embed Origins (optional)
              </label>
              <p class="text-xs text-muted">
                Comma-separated list of origins that can embed Pulse (leave empty for same-origin
                only)
              </p>
              <input
                type="text"
                value={props.allowedEmbedOrigins()}
                onChange={(e) => {
                  props.setAllowedEmbedOrigins(e.currentTarget.value);
                  props.setHasUnsavedChanges(true);
                }}
                placeholder="https://my.domain, https://dashboard.example.com"
                class="w-full min-h-10 sm:min-h-10 px-3 py-2.5 text-sm border rounded-md border-border bg-surface"
              />
              <p class="text-xs text-muted">
                Example: If Pulse is at <code>pulse.my.domain</code> and your dashboard is at{' '}
                <code>my.domain</code>, add <code>https://my.domain</code> here.
              </p>
            </div>
          </Show>
        </div>
      </section>

      <section class="p-4 sm:p-6 space-y-4">
        <h3 class="text-sm font-semibold text-base-content flex items-center gap-2">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 sm:h-4 sm:w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width={2}
              d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
            />
          </svg>
          Webhook Security
        </h3>
        <div class="space-y-3">
          <div>
            <label class="text-sm font-medium text-base-content">
              Allowed Private IP Ranges for Webhooks
            </label>
            <p class="text-xs text-muted mb-2">
              By default, webhooks to private IP addresses are blocked for security. Enter trusted
              CIDR ranges to allow webhooks to internal services (leave empty to block all private
              IPs).
            </p>
            <input
              type="text"
              value={props.webhookAllowedPrivateCIDRs()}
              onChange={(e) => {
                props.setWebhookAllowedPrivateCIDRs(e.currentTarget.value);
                props.setHasUnsavedChanges(true);
              }}
              placeholder="192.168.1.0/24, 10.0.0.0/8"
              class="w-full min-h-10 sm:min-h-10 px-3 py-2.5 text-sm border rounded-md border-border bg-surface"
            />
            <p class="text-xs text-muted mt-1">
              Example: <code>192.168.1.0/24,10.0.0.0/8</code> allows webhooks to these private
              networks. Localhost and cloud metadata services remain blocked.
            </p>
          </div>
        </div>
      </section>

      <div class="p-4 sm:p-6">
        <Card
          tone="warning"
          padding="sm"
          border={false}
          class="border border-amber-200 dark:border-amber-800"
        >
          <p class="text-xs text-amber-800 dark:text-amber-200 mb-2">
            <strong>Port Configuration:</strong> Use{' '}
            <code class="font-mono bg-amber-100 dark:bg-amber-800 px-1 rounded">
              systemctl edit pulse
            </code>
          </p>
          <p class="text-xs text-amber-700 dark:text-amber-300 font-mono">
            [Service]
            <br />
            Environment="FRONTEND_PORT=8080"
            <br />
            <span class="text-xs text-amber-600 dark:text-amber-400">
              Then restart: sudo systemctl restart pulse
            </span>
          </p>
        </Card>
      </div>
    </>
  );
};
