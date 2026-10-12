import { createResource, createSignal, onCleanup, Show } from 'solid-js';
import CheckCircleIcon from 'lucide-solid/icons/check-circle-2';
import CircleAlertIcon from 'lucide-solid/icons/circle-alert';
import { AIAPI } from '@/api/ai';
import { getPatrolCostPreview, getPatrolModelGuidance } from '@/api/aiPatrolCost';
import { runPatrolModelReadiness } from '@/api/patrol';
import { settingsTabPath } from '@/components/Settings/settingsNavigationModel';
import { getAIProviderConfig } from '@/components/Settings/aiSettingsModel';
import { Button } from '@/components/shared/Button';
import { ExternalTextLink } from '@/components/shared/ExternalTextLink';
import { formControl, formHelpText, formLabel } from '@/components/shared/Form';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { SelectionCardGroup } from '@/components/shared/SelectionCardGroup';
import { aiChatStore } from '@/stores/aiChat';
import { syncAIRuntimeSettings } from '@/stores/aiRuntimeState';
import { splitModelRoute } from '@/utils/aiPatrolCostPresentation';
import { logger } from '@/utils/logger';
import {
  buildPatrolSetupConnectRequest,
  buildPatrolSetupModelRequest,
  getPatrolSetupCheckVerdict,
  getPatrolSetupCostLine,
  getPatrolSetupInitialProvider,
  getPatrolSetupIntervalAdjust,
  getPatrolSetupNoModelMessage,
  getPatrolSetupOllamaHint,
  getPatrolSetupProviderFailure,
  getPatrolSetupSuggestedRoute,
  PATROL_SETUP_DEFAULT_OLLAMA_URL,
  PATROL_SETUP_PROVIDER_OPTIONS,
  pickPatrolSetupModel,
  type PatrolSetupCheckVerdict,
  type PatrolSetupPhase,
  type PatrolSetupProvider,
} from './patrolSetupModel';

const errorText = (error: unknown, fallback: string) =>
  error instanceof Error && error.message.trim() ? error.message.trim() : fallback;

export function PatrolSetupPanel(props: {
  /** Provider Patrol readiness names, such as the one whose key was refused. */
  readinessProvider?: string;
  /** Keep the panel mounted while setup runs, whatever polling reports. */
  onStart: () => void;
  /** Re-read Patrol state once setup is finished. */
  onDone: () => void;
  /** Re-read Patrol state, then start the first run. */
  onRunPatrol: () => void;
}) {
  const [provider, setProvider] = createSignal<PatrolSetupProvider>(
    getPatrolSetupInitialProvider(props.readinessProvider),
  );
  const [apiKey, setApiKey] = createSignal('');
  const [ollamaUrl, setOllamaUrl] = createSignal(PATROL_SETUP_DEFAULT_OLLAMA_URL);
  const [phase, setPhase] = createSignal<PatrolSetupPhase>('idle');
  const [error, setError] = createSignal('');
  const [modelId, setModelId] = createSignal('');
  const [verdict, setVerdict] = createSignal<PatrolSetupCheckVerdict | null>(null);
  let checkAbort: AbortController | undefined;
  onCleanup(() => checkAbort?.abort());

  const [guidance] = createResource(() => getPatrolModelGuidance().catch(() => null));
  const suggestedRoute = () => getPatrolSetupSuggestedRoute(provider(), guidance());
  const [costPreview] = createResource(suggestedRoute, (model) =>
    model ? getPatrolCostPreview({ model }).catch(() => null) : null,
  );
  const costLine = () => getPatrolSetupCostLine(costPreview(), costPreview()?.interval_minutes);
  const providerConfig = () => getAIProviderConfig(provider());
  const busy = () => phase() === 'connecting' || phase() === 'checking';
  const connectRequest = () =>
    buildPatrolSetupConnectRequest(provider(), { apiKey: apiKey(), ollamaUrl: ollamaUrl() });

  const selectProvider = (next: PatrolSetupProvider) => {
    setProvider(next);
    setApiKey('');
    setError('');
    setModelId('');
    setVerdict(null);
    if (phase() === 'failed') setPhase('idle');
  };

  const runCheck = async (model: string) => {
    checkAbort?.abort();
    const controller = new AbortController();
    checkAbort = controller;
    setPhase('checking');
    setError('');
    try {
      const result = await runPatrolModelReadiness({ model }, controller.signal);
      if (controller.signal.aborted) return;
      const next = getPatrolSetupCheckVerdict(result);
      setVerdict(next);
      setPhase(next.ready ? 'ready' : 'failed');
    } catch (err) {
      if (controller.signal.aborted) return;
      setVerdict(getPatrolSetupCheckVerdict(null));
      setError(errorText(err, ''));
      setPhase('failed');
    }
  };

  const connect = async (event: Event) => {
    event.preventDefault();
    const request = connectRequest();
    if (!request || busy()) return;
    props.onStart();
    setPhase('connecting');
    setError('');
    setVerdict(null);
    try {
      const connected = await AIAPI.updateSettings(request);
      syncAIRuntimeSettings(connected);
      void aiChatStore.refreshEnabledFromServer();
      // The key is saved now; keep it out of the page.
      setApiKey('');

      // The model list hides a rejected key behind an empty list, so ask the
      // provider directly first, the same test Provider & Models runs.
      const refused = getPatrolSetupProviderFailure(
        providerConfig().title,
        await AIAPI.testProvider(provider()),
      );
      if (refused) {
        setError(refused);
        setPhase('failed');
        return;
      }

      const listed = await AIAPI.getModels();
      const picked = pickPatrolSetupModel(provider(), listed.models ?? [], guidance());
      if (!picked) {
        setError(
          listed.error?.trim()
            ? `${providerConfig().title} did not return its models: ${listed.error.trim()}`
            : getPatrolSetupNoModelMessage(provider(), suggestedRoute()),
        );
        setPhase('failed');
        return;
      }

      const savedInterval = connected.patrol_interval_minutes ?? 360;
      const projection = await getPatrolCostPreview({ model: picked }).catch(() => null);
      const updated = await AIAPI.updateSettings(
        buildPatrolSetupModelRequest(picked, {
          sharedModel: connected.model ?? '',
          intervalAdjust: getPatrolSetupIntervalAdjust(projection, savedInterval),
        }),
      );
      syncAIRuntimeSettings(updated);
      setModelId(picked);
      await runCheck(picked);
    } catch (err) {
      logger.debug('[PatrolSetup] Setup failed', err);
      setError(errorText(err, 'Pulse could not save the provider settings.'));
      setPhase('failed');
    }
  };

  const modelLabel = () => splitModelRoute(modelId()).model;

  return (
    <div class="space-y-4" data-testid="patrol-setup-panel">
      <Show
        when={phase() !== 'ready'}
        fallback={
          <div class="space-y-3" role="status" data-testid="patrol-setup-ready">
            <InlineNotice tone="success" icon={<CheckCircleIcon />}>
              <p class="font-medium">{verdict()?.headline}</p>
              <p class="text-xs">
                {modelLabel()}: {verdict()?.detail}
              </p>
            </InlineNotice>
            <div class="flex flex-wrap gap-2">
              <Button type="button" variant="primary" size="sm" onClick={() => props.onRunPatrol()}>
                Run Patrol now
              </Button>
              <Button type="button" variant="secondary" size="sm" onClick={() => props.onDone()}>
                Done
              </Button>
            </div>
          </div>
        }
      >
        <form class="space-y-4" onSubmit={(event) => void connect(event)}>
          <p class="text-sm leading-6 text-base-content">
            Connect a model and Patrol starts checking your infrastructure. Pulse picks the model,
            shows what it costs, and tests it before Patrol runs.
          </p>
          <SelectionCardGroup
            options={PATROL_SETUP_PROVIDER_OPTIONS}
            value={provider()}
            onChange={selectProvider}
            variant="compact"
            disabled={busy()}
          />
          <Show
            when={provider() === 'ollama'}
            fallback={
              <div>
                <label for="patrol-setup-api-key" class={formLabel}>
                  {providerConfig().title} API key
                </label>
                <input
                  id="patrol-setup-api-key"
                  type="password"
                  autocomplete="off"
                  class={formControl}
                  value={apiKey()}
                  placeholder={providerConfig().placeholder}
                  disabled={busy()}
                  onInput={(event) => setApiKey(event.currentTarget.value)}
                />
                <p class={formHelpText}>
                  <ExternalTextLink href={providerConfig().actionLinkHref}>
                    Get an API key from {providerConfig().title}
                  </ExternalTextLink>
                </p>
              </div>
            }
          >
            <div>
              <label for="patrol-setup-ollama-url" class={formLabel}>
                Ollama server URL
              </label>
              <input
                id="patrol-setup-ollama-url"
                type="url"
                class={formControl}
                value={ollamaUrl()}
                disabled={busy()}
                onInput={(event) => setOllamaUrl(event.currentTarget.value)}
              />
              <p class={formHelpText}>{getPatrolSetupOllamaHint(suggestedRoute())}</p>
            </div>
          </Show>
          <Show when={suggestedRoute()}>
            <div class="text-xs leading-5 text-muted" data-testid="patrol-setup-cost">
              <p>
                Model:{' '}
                <span class="font-medium text-base-content">
                  {splitModelRoute(suggestedRoute()).model}
                </span>
              </p>
              <Show when={costLine()}>
                <p>{costLine()}</p>
              </Show>
            </div>
          </Show>

          <Show when={phase() === 'failed'}>
            <InlineNotice tone="danger" icon={<CircleAlertIcon />} data-testid="patrol-setup-error">
              <Show when={verdict()}>
                <p class="font-medium">{verdict()?.headline}</p>
                <p class="text-xs">{verdict()?.detail}</p>
              </Show>
              <Show when={error()}>
                <p class="text-xs">{error()}</p>
              </Show>
            </InlineNotice>
          </Show>
          <Show when={phase() === 'checking'}>
            <p class="text-xs text-muted" role="status">
              Testing {modelLabel()} with a Patrol check on sample data. This can take up to two
              minutes.
            </p>
          </Show>

          <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
            <Button
              type="submit"
              variant="primary"
              size="sm"
              isLoading={phase() === 'connecting'}
              disabled={busy() || !connectRequest()}
            >
              {phase() === 'connecting' ? 'Connecting' : 'Connect and check'}
            </Button>
            <Show when={modelId() && phase() !== 'connecting'}>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                isLoading={phase() === 'checking'}
                disabled={busy()}
                onClick={() => void runCheck(modelId())}
              >
                Check {modelLabel()} again
              </Button>
            </Show>
            <a
              href={settingsTabPath('system-ai')}
              class="text-xs font-medium text-blue-700 hover:underline dark:text-blue-300"
            >
              Other providers, subscriptions and model choice
            </a>
          </div>
        </form>
      </Show>
    </div>
  );
}

export default PatrolSetupPanel;
