import { createSignal, createEffect, Show, For } from 'solid-js';
import { NotificationsAPI } from '@/api/notifications';

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
  server: string;    // Fixed: use 'server' not 'smtpHost'
  port: number;      // Fixed: use 'port' not 'smtpPort'
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
  const [showProviders, setShowProviders] = createSignal(false);
  const [showAdvanced, setShowAdvanced] = createSignal(false);
  
  // Load email providers
  createEffect(async () => {
    try {
      const data = await NotificationsAPI.getEmailProviders();
      setProviders(data);
    } catch (err) {
      console.error('Failed to load email providers:', err);
    }
  });
  
  const selectProvider = (provider: EmailProvider) => {
    props.onChange({
      ...props.config,
      provider: provider.name,
      server: provider.smtpHost,  // Fixed: use 'server' not 'smtpHost'
      port: provider.smtpPort,    // Fixed: use 'port' not 'smtpPort'  
      tls: provider.tls,
      startTLS: provider.startTLS,
      username: provider.name === 'SendGrid' ? 'apikey' : props.config.username,
    });
    setShowProviders(false);
  };
  
  const currentProvider = () => providers().find(p => p.name === props.config.provider);
  
  return (
    <div class="space-y-6">
      {/* Provider Selection */}
      <div>
        <div class="flex items-center justify-between mb-4">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            Email Provider
          </label>
          <button type="button"
            onClick={() => setShowProviders(!showProviders())}
            class="text-sm text-blue-600 hover:text-blue-700 dark:text-blue-400"
          >
            {props.config.provider || 'Select Provider'} →
          </button>
        </div>
        
        <Show when={showProviders()}>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-2 p-4 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
            <For each={providers()}>
              {(provider) => (
                <button type="button"
                  onClick={() => selectProvider(provider)}
                  class={`p-3 text-left rounded-lg border transition-all ${
                    props.config.provider === provider.name
                      ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                      : 'border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700/30'
                  }`}
                >
                  <div class="font-medium text-sm text-gray-800 dark:text-gray-200">
                    {provider.name}
                  </div>
                  <div class="text-xs text-gray-600 dark:text-gray-400 mt-1">
                    {provider.smtpHost}:{provider.smtpPort}
                  </div>
                </button>
              )}
            </For>
          </div>
        </Show>
        
        <Show when={currentProvider()}>
          <div class="mt-4 p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
            <h4 class="text-sm font-medium text-blue-900 dark:text-blue-100 mb-2">
              Setup Instructions
            </h4>
            <pre class="text-xs text-blue-800 dark:text-blue-200 whitespace-pre-wrap">
              {currentProvider()!.instructions}
            </pre>
          </div>
        </Show>
      </div>
      
      {/* Basic Configuration */}
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            SMTP Server
          </label>
          <input
            type="text"
            value={props.config.server}
            onInput={(e) => props.onChange({ ...props.config, server: e.currentTarget.value })}
            placeholder="smtp.example.com"
            class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600"
          />
        </div>
        
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            SMTP Port
          </label>
          <input
            type="number"
            value={props.config.port}
            onInput={(e) => props.onChange({ ...props.config, port: parseInt(e.currentTarget.value) || 587 })}
            placeholder="587"
            class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600"
          />
        </div>
        
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            From Address
          </label>
          <input
            type="email"
            value={props.config.from}
            onInput={(e) => props.onChange({ ...props.config, from: e.currentTarget.value })}
            placeholder="noreply@example.com"
            class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600"
          />
        </div>
        
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Reply-To Address
          </label>
          <input
            type="email"
            value={props.config.replyTo || ''}
            onInput={(e) => props.onChange({ ...props.config, replyTo: e.currentTarget.value })}
            placeholder="admin@example.com (optional)"
            class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600"
          />
        </div>
        
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Username
          </label>
          <input
            type="text"
            value={props.config.username}
            onInput={(e) => props.onChange({ ...props.config, username: e.currentTarget.value })}
            placeholder={props.config.provider === 'SendGrid' ? 'apikey' : 'username@example.com'}
            class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600"
          />
        </div>
        
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Password / API Key
          </label>
          <input
            type="password"
            value={props.config.password}
            onInput={(e) => props.onChange({ ...props.config, password: e.currentTarget.value })}
            placeholder="••••••••"
            class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600"
          />
        </div>
      </div>
      
      {/* Recipients */}
      <div>
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
          Recipients (one per line)
          <span class="text-xs text-gray-500 dark:text-gray-400 ml-2">
            Leave empty to send to the From address
          </span>
        </label>
        <textarea
          value={props.config.to.join('\n')}
          onInput={(e) => {
            // Parse recipients - split by newlines and keep all non-empty lines
            const rawValue = e.currentTarget.value;
            const recipients = rawValue
              .split('\n')
              .map(r => r.trim())
              .filter(r => r.length > 0); // Keep all non-empty lines, validation happens on save
            props.onChange({ ...props.config, to: recipients });
          }}
          onKeyDown={(e) => {
            // Allow Enter key in textarea without triggering form submission
            if (e.key === 'Enter') {
              e.stopPropagation();
            }
          }}
          placeholder={`Leave empty to use ${props.config.from || 'From address'}\nOr add additional recipients:\nadmin@company.com\nops-team@company.com`}
          rows="3"
          class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600"
        />
      </div>
      
      {/* Advanced Settings */}
      <div>
        <button type="button"
          onClick={() => setShowAdvanced(!showAdvanced())}
          class="text-sm text-blue-600 hover:text-blue-700 dark:text-blue-400 flex items-center gap-1"
        >
          <svg class={`w-4 h-4 transition-transform ${showAdvanced() ? 'rotate-90' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
          Advanced Settings
        </button>
        
        <Show when={showAdvanced()}>
          <div class="mt-4 space-y-4 p-4 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
            <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div>
                <label class="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={props.config.tls}
                    onChange={(e) => props.onChange({ ...props.config, tls: e.currentTarget.checked })}
                    class="rounded border-gray-300 dark:border-gray-600 text-blue-600"
                  />
                  <span class="text-sm text-gray-700 dark:text-gray-300">Use TLS</span>
                </label>
              </div>
              
              <div>
                <label class="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={props.config.startTLS}
                    onChange={(e) => props.onChange({ ...props.config, startTLS: e.currentTarget.checked })}
                    class="rounded border-gray-300 dark:border-gray-600 text-blue-600"
                  />
                  <span class="text-sm text-gray-700 dark:text-gray-300">Use STARTTLS</span>
                </label>
              </div>
              
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Rate Limit
                </label>
                <div class="flex items-center gap-2">
                  <input
                    type="number"
                    value={props.config.rateLimit || 60}
                    onInput={(e) => props.onChange({ ...props.config, rateLimit: parseInt(e.currentTarget.value) })}
                    class="w-20 px-2 py-1 text-sm border rounded dark:bg-gray-700 dark:border-gray-600"
                  />
                  <span class="text-sm text-gray-600 dark:text-gray-400">/min</span>
                </div>
              </div>
            </div>
            
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Max Retries
                </label>
                <input
                  type="number"
                  value={props.config.maxRetries || 3}
                  min="0"
                  max="5"
                  onInput={(e) => props.onChange({ ...props.config, maxRetries: parseInt(e.currentTarget.value) })}
                  class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600"
                />
              </div>
              
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Retry Delay (seconds)
                </label>
                <input
                  type="number"
                  value={props.config.retryDelay || 5}
                  min="1"
                  max="60"
                  onInput={(e) => props.onChange({ ...props.config, retryDelay: parseInt(e.currentTarget.value) })}
                  class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600"
                />
              </div>
            </div>
          </div>
        </Show>
      </div>
      
      {/* Test Button */}
      <div class="flex justify-end">
        <button type="button"
          onClick={props.onTest}
          disabled={props.testing || !props.config.enabled}
          class="px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {props.testing ? 'Sending Test Email...' : 'Send Test Email'}
        </button>
      </div>
    </div>
  );
}