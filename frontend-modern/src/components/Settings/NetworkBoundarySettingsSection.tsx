import { Component, Show } from 'solid-js';
import { EnvironmentLockBadge } from '@/components/shared/EnvironmentLockBadge';
import { Toggle } from '@/components/shared/Toggle';
import {
  validateOriginList,
  validatePrivateCIDRList,
  validatePublicURL,
  type NetworkBoundarySettingsSectionProps,
} from './networkSettingsModel';

interface EnvironmentOverrideNoticeProps {
  envVar: string;
  description: string;
}

function EnvironmentOverrideNotice(props: EnvironmentOverrideNoticeProps) {
  return (
    <div class="mt-2 flex flex-wrap items-center gap-2 rounded-sm border border-amber-300 bg-amber-100 p-2 text-xs text-amber-800 dark:border-amber-700 dark:bg-amber-900/25 dark:text-amber-200">
      <EnvironmentLockBadge envVar={props.envVar} />
      <span>Overridden by {props.envVar} environment variable</span>
      <span class="w-full text-amber-700 dark:text-amber-300">{props.description}</span>
    </div>
  );
}

export const NetworkBoundarySettingsSection: Component<NetworkBoundarySettingsSectionProps> = (
  props,
) => {
  const publicURLError = () =>
    props.envOverrides().publicURL ? null : validatePublicURL(props.publicURL());
  const corsOriginsError = () =>
    props.envOverrides().allowedOrigins ? null : validateOriginList(props.allowedOrigins(), true);
  const embedOriginsError = () =>
    props.envOverrides().allowedEmbedOrigins
      ? null
      : validateOriginList(props.allowedEmbedOrigins(), false);
  const webhookCIDRsError = () =>
    props.envOverrides().webhookAllowedPrivateCIDRs
      ? null
      : validatePrivateCIDRList(props.webhookAllowedPrivateCIDRs());
  const isDocker = () => Boolean(props.versionInfo?.()?.isDocker);

  return (
    <>
      <section class="p-4 sm:p-6 space-y-4">
        <div class="flex flex-wrap items-center gap-2">
          <h4 class="flex items-center gap-2 text-sm font-medium text-base-content">
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              aria-hidden="true"
            >
              <path d="M10 13a5 5 0 007.54.54l3-3a5 5 0 00-7.07-7.07l-1.72 1.71"></path>
              <path d="M14 11a5 5 0 00-7.54-.54l-3 3a5 5 0 007.07 7.07l1.71-1.71"></path>
            </svg>
            Public URL
          </h4>
          <Show when={props.envOverrides().publicURL}>
            <EnvironmentLockBadge envVar="PULSE_PUBLIC_URL" />
          </Show>
        </div>
        <div class="space-y-2">
          <label for="pulse-public-url" class="text-sm font-medium text-base-content">
            Pulse URL for Alerts and Agent Commands
          </label>
          <p class="text-xs text-muted">
            Where email alert links and copied agent install/update commands point back to Pulse.
            Set this to the URL you actually use to reach Pulse, such as an HTTPS domain behind a
            reverse proxy. Required for Docker deployments with custom ports.
          </p>
          <div class="relative">
            <input
              id="pulse-public-url"
              type="text"
              value={props.publicURL()}
              aria-invalid={Boolean(publicURLError())}
              onInput={(e) => {
                if (!props.envOverrides().publicURL) {
                  props.setPublicURL(e.currentTarget.value);
                  props.setHasUnsavedChanges(true);
                }
              }}
              disabled={props.envOverrides().publicURL}
              placeholder="https://pulse.example.com"
              class={`w-full min-h-10 sm:min-h-10 px-3 py-2.5 text-sm border rounded-md text-base-content ${
                props.envOverrides().publicURL
                  ? 'border-amber-300 dark:border-amber-600 bg-amber-50 dark:bg-amber-900/25 cursor-not-allowed opacity-75'
                  : publicURLError()
                    ? 'border-red-400 bg-surface dark:border-red-500'
                    : 'border-border bg-surface'
              }`}
            />
            <Show when={publicURLError()}>
              {(message) => (
                <p role="alert" class="mt-1.5 text-xs text-red-600 dark:text-red-400">
                  {message()}
                </p>
              )}
            </Show>
            <Show when={props.envOverrides().publicURL}>
              <EnvironmentOverrideNotice
                envVar="PULSE_PUBLIC_URL"
                description="Remove the env var and restart to enable UI configuration."
              />
            </Show>
          </div>
          <p class="text-xs text-muted mt-1">
            Example: If you access Pulse at <code>https://pulse.example.com</code>, enter that URL
            here. Leave empty to auto-detect from each request (may not work correctly with Docker
            port mappings).
          </p>
        </div>
      </section>

      <section class="p-4 sm:p-6 space-y-4">
        <div class="flex flex-wrap items-center gap-2">
          <h4 class="flex items-center gap-2 text-sm font-medium text-base-content">
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              aria-hidden="true"
            >
              <circle cx="12" cy="12" r="10"></circle>
              <path d="M2 12h20M12 2a15.3 15.3 0 014 10 15.3 15.3 0 01-4 10 15.3 15.3 0 01-4-10 15.3 15.3 0 014-10z"></path>
            </svg>
            Cross-Origin Access (CORS)
          </h4>
          <Show when={props.envOverrides().allowedOrigins}>
            <EnvironmentLockBadge envVar="ALLOWED_ORIGINS" />
          </Show>
        </div>
        <div class="space-y-2">
          <label for="cors-allowed-origins" class="text-sm font-medium text-base-content">
            CORS Allowed Origins
          </label>
          <p id="cors-origin-help" class="text-xs text-muted">
            A same-origin reverse proxy needs no CORS exception. For a separate browser app, allow
            only its exact trusted origin (scheme, host and port), without a path or trailing slash.
            Separate origins with commas.
          </p>
          <p id="cors-origin-limits" class="text-xs text-muted">
            Empty grants no cross-origin browser permission. * allows any origin without
            credentialed browser access. It is not a login or proxy repair. CORS does not replace
            authentication, CSRF protection or TLS.
          </p>
          <div class="relative">
            <input
              id="cors-allowed-origins"
              type="text"
              aria-describedby="cors-origin-help cors-origin-limits"
              aria-invalid={Boolean(corsOriginsError())}
              value={props.allowedOrigins()}
              onInput={(e) => {
                if (!props.envOverrides().allowedOrigins) {
                  props.setAllowedOrigins(e.currentTarget.value);
                  props.setHasUnsavedChanges(true);
                }
              }}
              disabled={props.envOverrides().allowedOrigins}
              placeholder="https://app.example.com:8443"
              class={`w-full min-h-10 sm:min-h-10 px-3 py-2.5 text-sm border rounded-md text-base-content ${
                props.envOverrides().allowedOrigins
                  ? 'border-amber-300 dark:border-amber-600 bg-amber-50 dark:bg-amber-900/25 cursor-not-allowed opacity-75'
                  : corsOriginsError()
                    ? 'border-red-400 bg-surface dark:border-red-500'
                    : 'border-border bg-surface'
              }`}
            />
            <Show when={corsOriginsError()}>
              {(message) => (
                <p role="alert" class="mt-1.5 text-xs text-red-600 dark:text-red-400">
                  {message()}
                </p>
              )}
            </Show>
            <Show when={props.envOverrides().allowedOrigins}>
              <EnvironmentOverrideNotice
                envVar="ALLOWED_ORIGINS"
                description="Remove the env var and restart to enable UI configuration."
              />
            </Show>
          </div>
        </div>
      </section>

      <section class="p-4 sm:p-6 space-y-4">
        <div class="flex flex-wrap items-center gap-2">
          <h4 class="flex items-center gap-2 text-sm font-medium text-base-content">
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              aria-hidden="true"
            >
              <rect x="3" y="4" width="18" height="14" rx="2"></rect>
              <path d="M7 20h10"></path>
            </svg>
            Embedding
          </h4>
          <Show when={props.envOverrides().allowEmbedding}>
            <EnvironmentLockBadge envVar="ALLOW_EMBEDDING" />
          </Show>
        </div>
        <div class="space-y-3">
          <div class="flex items-center justify-between gap-4">
            <div>
              <p class="text-sm font-medium text-base-content">Allow iframe embedding</p>
              <p class="text-xs text-muted">
                Allow Pulse to be embedded in iframes (for example, Homepage).
              </p>
            </div>
            <Toggle
              checked={props.allowEmbedding()}
              disabled={Boolean(props.envOverrides().allowEmbedding)}
              ariaLabel="Allow iframe embedding"
              onChange={(event) => {
                if (props.envOverrides().allowEmbedding) return;
                props.setAllowEmbedding(event.currentTarget.checked);
                props.setHasUnsavedChanges(true);
              }}
            />
          </div>

          <Show when={props.allowEmbedding()}>
            <div class="space-y-2 pt-1">
              <div class="flex flex-wrap items-center gap-2">
                <label for="allowed-embed-origins" class="text-xs font-medium text-base-content">
                  Allowed Embed Origins (optional)
                </label>
                <Show when={props.envOverrides().allowedEmbedOrigins}>
                  <EnvironmentLockBadge envVar="ALLOWED_EMBED_ORIGINS" />
                </Show>
              </div>
              <p class="text-xs text-muted">
                Comma-separated list of origins that can embed Pulse (leave empty for same-origin
                only).
              </p>
              <input
                id="allowed-embed-origins"
                type="text"
                aria-invalid={Boolean(embedOriginsError())}
                disabled={Boolean(props.envOverrides().allowedEmbedOrigins)}
                value={props.allowedEmbedOrigins()}
                onInput={(e) => {
                  if (props.envOverrides().allowedEmbedOrigins) return;
                  props.setAllowedEmbedOrigins(e.currentTarget.value);
                  props.setHasUnsavedChanges(true);
                }}
                placeholder="https://my.domain, https://pulse.example.com"
                class={`w-full min-h-10 sm:min-h-10 px-3 py-2.5 text-sm border rounded-md text-base-content ${
                  props.envOverrides().allowedEmbedOrigins
                    ? 'border-amber-300 dark:border-amber-600 bg-amber-50 dark:bg-amber-900/25 cursor-not-allowed opacity-75'
                    : embedOriginsError()
                      ? 'border-red-400 bg-surface dark:border-red-500'
                      : 'border-border bg-surface'
                }`}
              />
              <Show when={embedOriginsError()}>
                {(message) => (
                  <p role="alert" class="mt-1 text-xs text-red-600 dark:text-red-400">
                    {message()}
                  </p>
                )}
              </Show>
              <p class="text-xs text-muted">
                Example: If Pulse is at <code>pulse.my.domain</code> and your homepage is at{' '}
                <code>my.domain</code>, add <code>https://my.domain</code> here.
              </p>
            </div>
          </Show>
        </div>
      </section>

      <section class="p-4 sm:p-6 space-y-4">
        <div class="flex flex-wrap items-center gap-2">
          <h4 class="flex items-center gap-2 text-sm font-medium text-base-content">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              aria-hidden="true"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width={2}
                d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
              />
            </svg>
            Webhook Security
          </h4>
          <Show when={props.envOverrides().webhookAllowedPrivateCIDRs}>
            <EnvironmentLockBadge envVar="WEBHOOK_ALLOWED_PRIVATE_CIDRS" />
          </Show>
        </div>
        <div class="space-y-3">
          <div>
            <label for="webhook-private-cidrs" class="text-sm font-medium text-base-content">
              Allowed Private IP Ranges for Webhooks
            </label>
            <p class="text-xs text-muted mb-2">
              By default, webhooks to private IP addresses are blocked for security. Enter trusted
              CIDR ranges to allow webhooks to internal services (leave empty to block all private
              IPs).
            </p>
            <input
              id="webhook-private-cidrs"
              type="text"
              aria-invalid={Boolean(webhookCIDRsError())}
              disabled={Boolean(props.envOverrides().webhookAllowedPrivateCIDRs)}
              value={props.webhookAllowedPrivateCIDRs()}
              onInput={(e) => {
                if (props.envOverrides().webhookAllowedPrivateCIDRs) return;
                props.setWebhookAllowedPrivateCIDRs(e.currentTarget.value);
                props.setHasUnsavedChanges(true);
              }}
              placeholder="192.168.1.0/24, 10.0.0.0/8"
              class={`w-full min-h-10 sm:min-h-10 px-3 py-2.5 text-sm border rounded-md text-base-content ${
                props.envOverrides().webhookAllowedPrivateCIDRs
                  ? 'border-amber-300 dark:border-amber-600 bg-amber-50 dark:bg-amber-900/25 cursor-not-allowed opacity-75'
                  : webhookCIDRsError()
                    ? 'border-red-400 bg-surface dark:border-red-500'
                    : 'border-border bg-surface'
              }`}
            />
            <Show when={webhookCIDRsError()}>
              {(message) => (
                <p role="alert" class="mt-1.5 text-xs text-red-600 dark:text-red-400">
                  {message()}
                </p>
              )}
            </Show>
            <p class="text-xs text-muted mt-1">
              Example: <code>192.168.1.0/24,10.0.0.0/8</code> allows webhooks to these private
              networks. Localhost and cloud metadata services remain blocked.
            </p>
          </div>
        </div>
      </section>

      <div class="p-4 sm:p-6">
        <details class="rounded-md border border-border bg-surface-alt p-3 text-xs text-muted">
          <summary class="cursor-pointer font-medium text-base-content">
            Port configuration (configured outside the UI)
          </summary>
          <Show
            when={isDocker()}
            fallback={
              <div class="mt-2 space-y-1.5">
                <p>
                  For systemd installs, override the listen port with{' '}
                  <code class="rounded-sm bg-surface px-1 py-0.5 font-mono text-base-content">
                    systemctl edit pulse
                  </code>
                  :
                </p>
                <pre class="overflow-x-auto rounded-sm border border-border bg-page p-2 font-mono text-base-content">
                  {`[Service]\nEnvironment="FRONTEND_PORT=8080"`}
                </pre>
                <p>
                  Then restart with{' '}
                  <code class="rounded-sm bg-surface px-1 py-0.5 font-mono text-base-content">
                    sudo systemctl restart pulse
                  </code>
                  .
                </p>
              </div>
            }
          >
            <div class="mt-2 space-y-1.5">
              <p>
                For Docker installs, change the published port mapping in your Compose file or{' '}
                <code class="rounded-sm bg-surface px-1 py-0.5 font-mono text-base-content">
                  docker run
                </code>{' '}
                command (for example{' '}
                <code class="rounded-sm bg-surface px-1 py-0.5 font-mono text-base-content">
                  -p 8080:7655
                </code>
                ), then recreate the container.
              </p>
            </div>
          </Show>
        </details>
      </div>
    </>
  );
};
