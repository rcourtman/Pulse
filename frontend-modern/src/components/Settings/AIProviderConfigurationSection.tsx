import { For, Show, type Accessor, type Component, type Setter } from 'solid-js';
import type { SetStoreFunction } from 'solid-js/store';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { CopyCommandBlock } from '@/components/Settings/CopyCommandBlock';
import { ExternalTextLink } from '@/components/shared/ExternalTextLink';
import { HelpIcon } from '@/components/shared/HelpIcon';
import { controlClass } from '@/components/shared/Form';
import { getAIProviderHealthPresentation } from '@/utils/aiProviderHealthPresentation';
import { getAIProviderDisplayName } from '@/utils/aiProviderPresentation';
import { getAIProviderTestResultTextClass } from '@/utils/aiSettingsPresentation';
import type { AIProvider, AISettings as AISettingsType } from '@/types/ai';
import {
  AI_PROVIDER_CONFIGS,
  AI_PROVIDERS,
  type AIProviderCredentialsFormState,
  type ProviderHealthState,
  type ProviderTestResult,
  isAIProviderConfigured,
} from '@/components/Settings/aiSettingsModel';

export interface AIProviderConfigurationSectionProps {
  settings: Accessor<AISettingsType | null>;
  form: AIProviderCredentialsFormState;
  setForm: SetStoreFunction<AIProviderCredentialsFormState>;
  expandedProviders: Accessor<Set<AIProvider>>;
  setExpandedProviders: Setter<Set<AIProvider>>;
  providerHealth: Record<AIProvider, ProviderHealthState>;
  preflightRunning: Accessor<boolean>;
  preflightLastCheckedAt: Accessor<number | null>;
  providerIssueCount: Accessor<number>;
  testingProvider: Accessor<AIProvider | null>;
  providerTestResult: Accessor<ProviderTestResult | null>;
  saving: Accessor<boolean>;
  runProviderPreflight: () => Promise<void>;
  handleTestProvider: (provider: AIProvider) => Promise<void>;
  handleClearProvider: (provider: AIProvider) => Promise<void>;
}

export const AIProviderConfigurationSection: Component<AIProviderConfigurationSectionProps> = (
  props,
) => {
  const toggleProvider = (provider: AIProvider) => {
    const next = new Set(props.expandedProviders());
    if (next.has(provider)) {
      next.delete(provider);
    } else {
      next.add(provider);
    }
    props.setExpandedProviders(next);
  };

  const isConfigured = (provider: AIProvider) => isAIProviderConfigured(provider, props.settings());

  const providerIssueProviders = () =>
    AI_PROVIDERS.filter((provider) => props.providerHealth[provider].status === 'error');

  return (
    <div class="p-5 rounded-md border border-border bg-surface-alt">
      <div class="mb-3 space-y-1.5">
        <div class="flex items-center justify-between gap-2">
          <h4 class="font-medium text-base-content flex items-center gap-2">
            <svg
              class="w-5 h-5 text-blue-600 dark:text-blue-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"
              />
            </svg>
            Provider Configuration
          </h4>
          <button
            type="button"
            onClick={() => void props.runProviderPreflight()}
            disabled={props.preflightRunning() || props.saving()}
            class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50"
          >
            {props.preflightRunning() ? 'Checking...' : 'Run Preflight'}
          </button>
        </div>
        <p class="text-xs text-muted mt-1">
          Configure API keys for each provider you want to use. Models from all configured providers
          will appear in the model selectors.
        </p>
        <Show when={props.preflightLastCheckedAt()}>
          <p class="text-[11px] text-muted">
            Last checked: {new Date(props.preflightLastCheckedAt()!).toLocaleTimeString()}
          </p>
        </Show>
        <Show when={props.providerIssueCount() > 0}>
          <CalloutCard
            tone="danger"
            scale="compact"
            padding="sm"
            title={`${props.providerIssueCount()} provider${
              props.providerIssueCount() === 1 ? '' : 's'
            } configured but currently not usable.`}
          >
            <div class="space-y-1 text-[11px]">
              <For each={providerIssueProviders()}>
                {(provider) => (
                  <p>
                    <span class="font-medium">
                      {getAIProviderDisplayName(provider) || provider}:
                    </span>{' '}
                    {props.providerHealth[provider].message}
                  </p>
                )}
              </For>
            </div>
          </CalloutCard>
        </Show>
      </div>

      <div class="space-y-2">
        <For each={AI_PROVIDER_CONFIGS}>
          {(config) => {
            const configured = () => isConfigured(config.provider);
            const expanded = () => props.expandedProviders().has(config.provider);
            const health = () => props.providerHealth[config.provider];
            const testResult = () =>
              props.providerTestResult()?.provider === config.provider
                ? props.providerTestResult()
                : null;
            const registryDefinition = () =>
              props.settings()?.providers?.find((def) => def.id === config.provider);
            const suggestedModel = () => registryDefinition()?.suggested_model ?? '';
            const suggestedModelNote = () => registryDefinition()?.suggested_model_note ?? '';
            const testedModelIsSuggested = () => {
              const definition = registryDefinition();
              if (!definition?.suggested_model) {
                return true;
              }
              const tested = (testResult()?.model ?? '').toLowerCase();
              if (!tested) {
                return true;
              }
              const bare = tested.startsWith(`${config.provider}:`)
                ? tested.slice(config.provider.length + 1)
                : tested;
              return [
                definition.suggested_model,
                ...(definition.suggested_model_equivalents ?? []),
              ].some((id) => id.toLowerCase() === bare);
            };

            return (
              <div
                class={`border rounded-md overflow-hidden ${configured() ? 'border-green-300 dark:border-green-700' : 'border-border'}`}
              >
                <button
                  type="button"
                  class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-surface hover:bg-surface-hover transition-colors"
                  onClick={() => toggleProvider(config.provider)}
                  aria-expanded={expanded()}
                  aria-controls={`${config.provider}-provider-panel`}
                >
                  <div class="flex items-center gap-2">
                    <span class="font-medium text-sm">{config.title}</span>
                    <Show when={configured()}>
                      <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">
                        {config.configuredLabel}
                      </span>
                    </Show>
                    <Show when={configured()}>
                      <span
                        class={`px-1.5 py-0.5 text-[10px] font-semibold rounded ${getAIProviderHealthPresentation(health().status).badgeClass}`}
                      >
                        {getAIProviderHealthPresentation(health().status).label}
                      </span>
                    </Show>
                  </div>
                  <svg
                    class={`w-4 h-4 transition-transform ${expanded() ? 'rotate-180' : ''}`}
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M19 9l-7 7-7-7"
                    />
                  </svg>
                </button>
                <Show when={expanded()}>
                  <div
                    id={`${config.provider}-provider-panel`}
                    class="px-3 py-3 bg-surface-alt border-t border-border space-y-2"
                  >
                    <Show when={config.provider === 'ollama'}>
                      <label class="text-xs text-muted inline-flex items-center gap-1">
                        Server URL
                        <HelpIcon contentId="ai.ollama.baseUrl" size="xs" />
                      </label>
                    </Show>
                    <input
                      type={config.inputType}
                      value={props.form[config.inputField]}
                      onInput={(event) =>
                        props.setForm(config.inputField, event.currentTarget.value)
                      }
                      placeholder={
                        configured() && config.configuredPlaceholder
                          ? config.configuredPlaceholder
                          : config.placeholder
                      }
                      aria-label={`${getAIProviderDisplayName(config.provider)} ${
                        config.inputType === 'url' ? 'server URL' : 'API key'
                      }`}
                      class={controlClass()}
                      disabled={props.saving()}
                    />

                    <For each={config.extraFields || []}>
                      {(extraField) => (
                        <div class="space-y-1">
                          <label class="text-xs text-muted inline-flex items-center gap-1">
                            {extraField.label}
                            <Show when={extraField.helpContentId}>
                              <HelpIcon contentId={extraField.helpContentId!} size="xs" />
                            </Show>
                          </label>
                          <input
                            type={extraField.type || 'text'}
                            value={props.form[extraField.inputField]}
                            onInput={(event) =>
                              props.setForm(extraField.inputField, event.currentTarget.value)
                            }
                            placeholder={extraField.placeholder}
                            aria-label={`${getAIProviderDisplayName(config.provider)} ${extraField.label}`}
                            class={controlClass()}
                            disabled={props.saving()}
                          />
                          <Show when={extraField.helperText}>
                            <p class="text-xs text-muted">{extraField.helperText}</p>
                          </Show>
                        </div>
                      )}
                    </For>

                    <Show when={config.provider === 'ollama' && suggestedModel()}>
                      <div class="space-y-1">
                        <p class="text-xs text-muted">
                          Patrol needs a model that can call tools. This one is tested with Pulse:
                        </p>
                        <CopyCommandBlock
                          command={`ollama pull ${suggestedModel()}`}
                          codeClass="block whitespace-pre overflow-x-auto rounded-md border border-border bg-base p-3 pr-10 font-mono text-xs text-base-content"
                        />
                        <Show when={suggestedModelNote()}>
                          <p class="text-xs text-muted">{suggestedModelNote()}</p>
                        </Show>
                      </div>
                    </Show>

                    <Show when={config.helperText}>
                      <p class="text-xs text-muted">{config.helperText}</p>
                    </Show>

                    <div class="flex items-center justify-between">
                      <p class="text-xs text-muted">
                        <ExternalTextLink href={config.actionLinkHref} variant="compact">
                          {config.actionLinkLabel}
                        </ExternalTextLink>
                        <Show when={config.actionLinkSuffix}>
                          <span class="text-slate-400">{config.actionLinkSuffix}</span>
                        </Show>
                      </p>
                      <Show when={configured()}>
                        <div class="flex gap-1">
                          <button
                            type="button"
                            onClick={() => void props.handleTestProvider(config.provider)}
                            disabled={props.testingProvider() === config.provider || props.saving()}
                            class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50"
                          >
                            {props.testingProvider() === config.provider ? 'Testing...' : 'Test'}
                          </button>
                          <button
                            type="button"
                            onClick={() => void props.handleClearProvider(config.provider)}
                            disabled={props.saving()}
                            class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-800 disabled:opacity-50"
                            title={config.clearTitle}
                          >
                            Clear
                          </button>
                        </div>
                      </Show>
                    </div>

                    <Show when={testResult()}>
                      <p
                        class={`text-xs ${getAIProviderTestResultTextClass(Boolean(testResult()?.success))}`}
                      >
                        {testResult()?.message}
                      </p>
                    </Show>
                    <Show
                      when={
                        config.provider === 'ollama' &&
                        testResult()?.success &&
                        !testedModelIsSuggested()
                      }
                    >
                      <p class="text-xs text-amber-600 dark:text-amber-400">
                        Next step for Patrol: run the ollama pull command above, then pick{' '}
                        {suggestedModel()} as the Patrol model.
                      </p>
                    </Show>
                  </div>
                </Show>
              </div>
            );
          }}
        </For>
      </div>
    </div>
  );
};

export default AIProviderConfigurationSection;
