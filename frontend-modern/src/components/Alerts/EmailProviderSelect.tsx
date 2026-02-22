import { createSignal, createEffect, Show, For } from 'solid-js';
import { NotificationsAPI } from '@/api/notifications';
import { logger } from '@/utils/logger';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';

interface EmailProvider {
  name: string;
  smtpHost: string;
  smtpPort: number;
  tls: boolean;
  startTLS: boolean;
  authRequired: boolean;
  instructions: string;
}

interface EmailConfig {
  enabled: boolean;
  provider: string;
  server: string;
  port: number;
  from: string;
  username: string;
  password: string;
  to: string[];
  tls: boolean;
  startTLS: boolean;
  replyTo: string;
  maxRetries: number;
  retryDelay: number;
  rateLimit: number;
}

interface EmailProviderSelectProps {
  config: EmailConfig;
  onChange: (config: EmailConfig) => void;
  onTest: () => void;
  testing?: boolean;
}

export function EmailProviderSelect(props: EmailProviderSelectProps) {
  const [providers, setProviders] = createSignal<EmailProvider[]>([]);
  const [showAdvanced, setShowAdvanced] = createSignal(false);
  const [showInstructions, setShowInstructions] = createSignal(false);

  // Load email providers once
  createEffect(async () => {
    try {
      const data = await NotificationsAPI.getEmailProviders();
      setProviders(data);
    } catch (err) {
      logger.error('Failed to load email providers', err);
    }
  });

  const applyProvider = (provider: EmailProvider | undefined) => {
    if (!provider) {
      props.onChange({ ...props.config, provider: '' });
      setShowInstructions(false);
      return;
    }

    props.onChange({
      ...props.config,
      provider: provider.name,
      server: provider.smtpHost,
      port: provider.smtpPort,
      tls: provider.tls,
      startTLS: provider.startTLS,
      username: provider.name === 'SendGrid' ? 'apikey' : props.config.username,
    });
    setShowInstructions(true);
  };

  const handleProviderChange = (value: string) => {
    if (!value) {
      applyProvider(undefined);
      return;
    }
    const provider = providers().find((p) => p.name === value);
    applyProvider(provider);
  };

  const currentProvider = () => providers().find((p) => p.name === props.config.provider);
  const instructionBoxClass = "mt-2 rounded border border-blue-200 bg-blue-50 px-3 py-2 text-xs leading-relaxed text-blue-900 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200";

  return (
    <div class="space-y-4 text-sm overflow-hidden">
      <div class={formField}>
        <label class={labelClass()}>Email provider</label>
        <div class="flex w-full flex-wrap items-center gap-2 sm:flex-nowrap">
          <select
            value={props.config.provider}
            onChange={(e) => handleProviderChange(e.currentTarget.value)}
            class={`${controlClass('px-2 py-1.5')} sm:w-auto sm:min-w-[180px]`}
          >
            <option value="">Manual configuration</option>
            <For each={providers()}>
              {(provider) => (
                <option value={provider.name}>
                  {provider.name} ({provider.smtpHost}:{provider.smtpPort})
                </option>
              )}
            </For>
          </select>
          <Show when={props.config.provider}>
            <button
              type="button"
              onClick={() => {
                const provider = currentProvider();
                if (provider) applyProvider(provider);
              }}
              class="text-xs font-medium text-blue-600 hover:underline dark:text-blue-400"
            >
              Reapply defaults
            </button>
          </Show>
        </div>
      </div>

      <Show when={currentProvider()}>
        <div class="sm:hidden w-full">
          <button
            type="button"
            onClick={() => setShowInstructions(!showInstructions())}
            class="text-xs font-medium text-blue-600 hover:underline dark:text-blue-300"
          >
            {showInstructions() ? 'Hide setup instructions' : 'Show setup instructions'}
          </button>
          <Show when={showInstructions()}>
            <div class={instructionBoxClass}>
              {currentProvider()!.instructions}
            </div>
          </Show>
        </div>
        <div class="hidden w-full sm:block">
          <div class={instructionBoxClass}>
            {currentProvider()!.instructions}
          </div>
        </div>
      </Show>

      <div class="grid w-full gap-3 sm:grid-cols-2">
        <div class={formField}>
          <label class={labelClass()}>SMTP server</label>
          <input
            type="text"
            value={props.config.server}
            onInput={(e) => props.onChange({ ...props.config, server: e.currentTarget.value })}
            placeholder="smtp.example.com"
            class={controlClass('px-2 py-1.5')}
          />
        </div>

        <div class={formField}>
          <label class={labelClass()}>SMTP port</label>
          <input
            type="number"
            value={props.config.port || ''}
            onInput={(e) => {
              const value = e.currentTarget.value;
              // Allow empty field while typing, parse as number when valid
              const port = value === '' ? 0 : parseInt(value, 10);
              props.onChange({ ...props.config, port: isNaN(port) ? 0 : port });
            }}
            onBlur={(e) => {
              // Apply default on blur if empty or invalid
              const value = parseInt(e.currentTarget.value, 10);
              if (!value || isNaN(value)) {
                props.onChange({ ...props.config, port: 587 });
              }
            }}
            placeholder="587"
            class={controlClass('px-2 py-1.5')}
          />
        </div>

        <div class={formField}>
          <label class={labelClass()}>From address</label>
          <input
            type="email"
            value={props.config.from}
            onInput={(e) => props.onChange({ ...props.config, from: e.currentTarget.value })}
            placeholder="noreply@example.com"
            class={controlClass('px-2 py-1.5')}
          />
        </div>

        <div class={formField}>
          <label class={labelClass()}>Reply-to address</label>
          <input
            type="email"
            value={props.config.replyTo || ''}
            onInput={(e) => props.onChange({ ...props.config, replyTo: e.currentTarget.value })}
            placeholder="admin@example.com"
            class={controlClass('px-2 py-1.5')}
          />
        </div>

        <div class={formField}>
          <label class={labelClass()}>Username</label>
          <input
            type="text"
            value={props.config.username}
            onInput={(e) => props.onChange({ ...props.config, username: e.currentTarget.value })}
            placeholder={props.config.provider === 'SendGrid' ? 'apikey' : 'username@example.com'}
            class={controlClass('px-2 py-1.5')}
          />
        </div>

        <div class={formField}>
          <label class={labelClass()}>Password / API key</label>
          <input
            type="password"
            value={props.config.password}
            onInput={(e) => props.onChange({ ...props.config, password: e.currentTarget.value })}
            placeholder="••••••••"
            class={controlClass('px-2 py-1.5')}
          />
        </div>
      </div>

      <div class={formField}>
        <label class={labelClass()}>Recipients (one per line)</label>
        <textarea
          value={props.config.to.join('\n')}
          onInput={(e) => {
            const recipients = e.currentTarget.value
              .split('\n')
              .map((entry) => entry.trim())
              .filter((entry) => entry.length > 0);
            props.onChange({ ...props.config, to: recipients });
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.stopPropagation();
            }
          }}
          rows={3}
          class={controlClass('px-2 py-1.5 font-mono leading-snug')}
          placeholder={`Leave empty to use ${props.config.from || 'the from address'}\nOr add one recipient per line`}
        />
      </div>

      <div class="border-t border-border pt-3">
        <button
          type="button"
          onClick={() => setShowAdvanced(!showAdvanced())}
          class="text-xs font-semibold uppercase tracking-wide transition-colors hover:text-muted"
        >
          {showAdvanced() ? 'Hide advanced options' : 'Show advanced options'}
        </button>

        <Show when={showAdvanced()}>
          <div class="mt-3 space-y-3 text-xs text-base-content">
            <div class="grid gap-3 sm:grid-cols-3">
              <div class="flex items-center gap-2">
                <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>Security</label>
                <select
                  value={props.config.tls ? 'tls' : props.config.startTLS ? 'starttls' : 'none'}
                  onChange={(e) => {
                    const value = e.currentTarget.value;
                    props.onChange({
                      ...props.config,
                      tls: value === 'tls',
                      startTLS: value === 'starttls',
                    });
                  }}
                  class={`${controlClass('px-2 py-1 text-sm')} min-w-[120px]`}
                >
                  <option value="none">None</option>
                  <option value="starttls">STARTTLS (587)</option>
                  <option value="tls">TLS/SSL (465)</option>
                </select>
              </div>
              <div class="flex w-full flex-wrap items-center gap-2 sm:flex-nowrap">
                <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>Rate limit</label>
                <input
                  type="number"
                  value={props.config.rateLimit || 60}
                  onInput={(e) =>
                    props.onChange({ ...props.config, rateLimit: parseInt(e.currentTarget.value) })
                  }
                  class={`${controlClass('px-2 py-1 text-sm')} w-20`}
                />
                <span class={formHelpText}>/min</span>
              </div>
            </div>

            <div class="grid w-full gap-3 sm:grid-cols-2">
              <div class={formField}>
                <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>Max retries</label>
                <input
                  type="number"
                  value={props.config.maxRetries || 3}
                  min={0}
                  max={5}
                  onInput={(e) =>
                    props.onChange({ ...props.config, maxRetries: parseInt(e.currentTarget.value) })
                  }
                  class={controlClass('px-2 py-1 text-sm')}
                />
              </div>
              <div class={formField}>
                <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                  Retry delay (seconds)
                </label>
                <input
                  type="number"
                  value={props.config.retryDelay || 5}
                  min={1}
                  max={60}
                  onInput={(e) =>
                    props.onChange({ ...props.config, retryDelay: parseInt(e.currentTarget.value) })
                  }
                  class={controlClass('px-2 py-1 text-sm')}
                />
              </div>
            </div>
          </div>
        </Show>
      </div>

      <div class="flex justify-end pt-2">
        <button
          type="button"
          onClick={props.onTest}
          disabled={props.testing || !props.config.enabled}
          class="rounded border border-blue-500 px-3 py-1.5 text-xs font-medium text-blue-600 transition-colors hover:bg-blue-50 disabled:opacity-50 disabled:cursor-not-allowed dark:border-blue-400 dark:text-blue-300 dark:hover:bg-blue-900"
        >
          {props.testing ? 'Sending test email…' : 'Send test email'}
        </button>
      </div>
    </div>
  );
}
