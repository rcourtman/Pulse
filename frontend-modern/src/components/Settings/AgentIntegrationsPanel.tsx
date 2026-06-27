import {
  Component,
  createEffect,
  createMemo,
  createResource,
  createSignal,
  For,
  onCleanup,
  onMount,
  Show,
} from 'solid-js';
import {
  AGENT_MCP_TOKEN_PLACEHOLDER,
  AGENT_SURFACE_ID_PULSE_MCP,
  AGENT_WORKFLOW_PROMPT_OPERATIONS_LOOP,
  fetchAgentCapabilitiesManifest,
  formatAgentMCPServersConfig,
  formatAgentOpenCodeMCPConfig,
  getAgentCapabilityErrorCodeSummaries,
  getAgentWorkflowPrompts,
  getAgentManifestSurfaceToolContract,
  getAgentMCPConfigFamilyByShape,
  getAgentSurfaceToolPosturePresentation,
  getAgentSurfaceContractEntries,
  groupAgentCapabilitiesByManifestCategories,
  normalizeAgentMCPAdapter,
} from '@/api/agentCapabilities';
import { Button, ButtonLink } from '@/components/shared/Button';
import { ExternalTextLink } from '@/components/shared/ExternalTextLink';
import SettingsPanel from '@/components/shared/SettingsPanel';
import {
  EXTERNAL_AGENT_SETUP_ANCHOR,
  PATROL_CONTROL_PATH,
  isExternalAgentSetupHash,
  PULSE_MCP_SETUP_ANCHOR,
  PULSE_MCP_TOKEN_SETUP_PATH,
} from '@/routing/resourceLinks';
import { AGENT_SUBSTRATE_DOC_URL } from '@/utils/docsLinks';
import KeyRoundIcon from 'lucide-solid/icons/key-round';
import SettingsIcon from 'lucide-solid/icons/settings';
import CopyCommandBlock from './CopyCommandBlock';
import { API_TOKEN_PATROL_EXTERNAL_AGENT_PRESET_LABEL } from './apiTokenManagerModel';

// All three external resources (the MCP adapter README, the agent-probe HTTP
// example, the substrate design notes) are consolidated under the shipped
// AGENT_SUBSTRATE.md doc, which links out to the cmd/* source files. Avoids
// frontend-runtime links pointing at unpinned GitHub main paths.

export const AgentIntegrationsPanel: Component = () => {
  const [manifest] = createResource(fetchAgentCapabilitiesManifest);
  const [copied, setCopied] = createSignal<string | null>(null);
  const [routeHash, setRouteHash] = createSignal('');
  const [panelHighlight, setPanelHighlight] = createSignal(false);
  const [installerCommandsOpen, setInstallerCommandsOpen] = createSignal(false);
  const [clientConfigOpen, setClientConfigOpen] = createSignal(false);
  const [advancedClientDetailsOpen, setAdvancedClientDetailsOpen] = createSignal(false);
  const [setupOpen, setSetupOpen] = createSignal(false);
  const origin = typeof window !== 'undefined' ? window.location.origin : 'http://localhost:7655';
  const mcpInstallShellCommand =
    'curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install-mcp.sh | bash';
  const mcpInstallPowerShellCommand =
    'irm https://github.com/rcourtman/Pulse/releases/latest/download/install-mcp.ps1 | iex';
  let highlightTimer: number | undefined;
  let routeFocusTimer: number | undefined;
  const mcpAdapter = createMemo(() => normalizeAgentMCPAdapter(manifest()?.mcpAdapter));
  const openCodeConfigFamily = createMemo(() =>
    getAgentMCPConfigFamilyByShape(mcpAdapter(), 'opencode_mcp'),
  );
  const mcpServersConfigFamily = createMemo(() =>
    getAgentMCPConfigFamilyByShape(mcpAdapter(), 'mcp_servers'),
  );
  const claudeMcpConfig = createMemo(() => formatAgentMCPServersConfig(mcpAdapter(), origin));
  const openCodeMcpConfig = createMemo(() => formatAgentOpenCodeMCPConfig(mcpAdapter(), origin));
  const requiredScopes = createMemo(() =>
    (manifest()?.requiredScopes ?? []).map((scope) => scope.trim()).filter(Boolean),
  );
  const surfaceContractEntries = createMemo(() => getAgentSurfaceContractEntries(manifest()));
  const workflowPrompts = createMemo(() => getAgentWorkflowPrompts(manifest()));
  const mcpSurfaceToolPosture = createMemo(() =>
    getAgentSurfaceToolPosturePresentation(
      getAgentManifestSurfaceToolContract(manifest(), AGENT_SURFACE_ID_PULSE_MCP),
    ),
  );
  const errorCodeSummaries = createMemo(() => getAgentCapabilityErrorCodeSummaries(manifest()));
  const grouped = createMemo(() => groupAgentCapabilitiesByManifestCategories(manifest()));
  const hasManifestInventory = createMemo(
    () =>
      workflowPrompts().length > 0 ||
      requiredScopes().length > 0 ||
      errorCodeSummaries().length > 0 ||
      Boolean(manifest() && grouped().length > 0),
  );
  const hasClientBuilderDetails = createMemo(
    () => surfaceContractEntries().length > 0 || hasManifestInventory(),
  );

  let copySnippetTimer: ReturnType<typeof setTimeout> | undefined;
  const handleCopySnippet = (snippet: string) => {
    setCopied(snippet);
    if (copySnippetTimer) clearTimeout(copySnippetTimer);
    copySnippetTimer = setTimeout(() => setCopied(null), 2000);
  };
  onCleanup(() => {
    if (copySnippetTimer) clearTimeout(copySnippetTimer);
  });

  const readRouteHash = () => (typeof window === 'undefined' ? '' : window.location.hash);

  const getPanelElement = () =>
    typeof document === 'undefined'
      ? null
      : (document.getElementById(EXTERNAL_AGENT_SETUP_ANCHOR) as HTMLElement | null);

  const findScrollableAncestor = (element: HTMLElement): HTMLElement | null => {
    let current = element.parentElement;
    while (current) {
      const style = window.getComputedStyle(current);
      const canScroll =
        (style.overflowY === 'auto' || style.overflowY === 'scroll') &&
        current.scrollHeight > current.clientHeight;
      if (canScroll) {
        return current;
      }
      current = current.parentElement;
    }
    return null;
  };

  const focusPanel = (): boolean => {
    const panelElement = getPanelElement();
    if (!panelElement) return false;
    if (typeof panelElement.scrollIntoView === 'function') {
      panelElement.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
    const scrollParent = findScrollableAncestor(panelElement);
    if (scrollParent && typeof scrollParent.scrollTo === 'function') {
      const parentRect = scrollParent.getBoundingClientRect();
      const panelRect = panelElement.getBoundingClientRect();
      scrollParent.scrollTo({
        top: Math.max(0, scrollParent.scrollTop + panelRect.top - parentRect.top - 16),
        behavior: 'smooth',
      });
    }
    setPanelHighlight(true);
    window.clearTimeout(highlightTimer);
    highlightTimer = window.setTimeout(() => setPanelHighlight(false), 1600);
    const panelRect = panelElement.getBoundingClientRect();
    return panelRect.top >= 0 && panelRect.top < 220;
  };

  const focusPanelUntilLayoutSettles = (hash: string, attempt = 0) => {
    if (readRouteHash() !== hash) return;
    const panelReached = focusPanel();
    if (panelReached || attempt >= 8) return;
    window.clearTimeout(routeFocusTimer);
    routeFocusTimer = window.setTimeout(
      () => focusPanelUntilLayoutSettles(hash, attempt + 1),
      attempt < 2 ? 100 : 250,
    );
  };

  onMount(() => {
    const updateRouteHash = () => setRouteHash(readRouteHash());
    updateRouteHash();
    window.addEventListener('hashchange', updateRouteHash);
    window.addEventListener('popstate', updateRouteHash);
    onCleanup(() => {
      window.removeEventListener('hashchange', updateRouteHash);
      window.removeEventListener('popstate', updateRouteHash);
    });
  });

  onCleanup(() => {
    if (highlightTimer) window.clearTimeout(highlightTimer);
    if (routeFocusTimer) window.clearTimeout(routeFocusTimer);
  });

  createEffect(() => {
    const hash = routeHash();
    if (!isExternalAgentSetupHash(hash) || !getPanelElement()) {
      return;
    }

    setSetupOpen(true);
    window.clearTimeout(routeFocusTimer);
    const canonicalHash = `#${EXTERNAL_AGENT_SETUP_ANCHOR}`;
    if (hash === `#${PULSE_MCP_SETUP_ANCHOR}` && window.location.hash === hash) {
      window.history.replaceState(
        window.history.state,
        '',
        `${window.location.pathname}${window.location.search}${canonicalHash}`,
      );
      setRouteHash(canonicalHash);
      focusPanelUntilLayoutSettles(canonicalHash);
      return;
    }

    focusPanelUntilLayoutSettles(hash);
  });

  return (
    <SettingsPanel
      id={EXTERNAL_AGENT_SETUP_ANCHOR}
      title="External agents"
      noPadding
      class={`scroll-mt-20 transition-shadow ${
        panelHighlight() ? 'ring-2 ring-blue-500 shadow-sm' : ''
      }`}
    >
      <span id={PULSE_MCP_SETUP_ANCHOR} class="sr-only" aria-hidden="true" />
      <div class="space-y-5 p-4 sm:p-6">
        <div class="flex flex-col gap-3 text-sm text-muted lg:flex-row lg:items-start lg:justify-between">
          <div class="max-w-3xl space-y-2">
            <p>Connect external tools to read Pulse context and request Patrol work.</p>
            <p>Patrol mode and scoped tokens control what connected agents can do.</p>
          </div>
          <div class="flex shrink-0 flex-wrap gap-2">
            <Show when={!setupOpen()}>
              <ButtonLink
                href={PATROL_CONTROL_PATH}
                variant="secondary"
                size="settingsAction"
                class="w-fit gap-2 font-semibold"
              >
                <SettingsIcon class="h-4 w-4" aria-hidden="true" />
                Choose Patrol mode
              </ButtonLink>
            </Show>
            <Button
              variant="secondary"
              size="settingsAction"
              class="w-fit font-semibold"
              aria-controls="external-agent-setup-details"
              aria-expanded={setupOpen()}
              onClick={() => setSetupOpen((open) => !open)}
            >
              {setupOpen() ? 'Hide connector setup' : 'Show connector setup'}
            </Button>
          </div>
        </div>

        <Show when={setupOpen()}>
          <div id="external-agent-setup-details" class="space-y-3">
            <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <h3 class="text-sm font-semibold text-base-content">Connector setup</h3>
            </div>
            <ol class="divide-y divide-border rounded-md border border-border text-sm">
              <li class="flex flex-col gap-3 p-3 sm:flex-row sm:items-center sm:justify-between">
                <div class="space-y-1">
                  <p class="text-[11px] font-semibold uppercase tracking-wide text-muted">Step 1</p>
                  <p class="font-semibold text-base-content">Choose Patrol mode</p>
                  <p class="text-xs text-muted">Required before agents can request Patrol work.</p>
                </div>
                <ButtonLink
                  href={PATROL_CONTROL_PATH}
                  variant="secondary"
                  size="settingsAction"
                  class="w-fit shrink-0 gap-2 font-semibold"
                >
                  <SettingsIcon class="h-4 w-4" aria-hidden="true" />
                  Choose Patrol mode
                </ButtonLink>
              </li>
              <li class="flex flex-col gap-3 p-3 sm:flex-row sm:items-center sm:justify-between">
                <div class="space-y-1">
                  <p class="text-[11px] font-semibold uppercase tracking-wide text-muted">Step 2</p>
                  <p class="font-semibold text-base-content">Create a scoped token</p>
                  <p class="text-xs text-muted">
                    Create a token with the{' '}
                    <span class="font-medium text-base-content">
                      {API_TOKEN_PATROL_EXTERNAL_AGENT_PRESET_LABEL}
                    </span>{' '}
                    preset.
                  </p>
                </div>
                <ButtonLink
                  href={PULSE_MCP_TOKEN_SETUP_PATH}
                  hardNavigation
                  variant="secondary"
                  size="settingsAction"
                  class="w-fit shrink-0 gap-2 font-semibold"
                >
                  <KeyRoundIcon class="h-4 w-4" aria-hidden="true" />
                  Create token
                </ButtonLink>
              </li>
              <li class="space-y-1 p-3">
                <p class="text-[11px] font-semibold uppercase tracking-wide text-muted">Step 3</p>
                <p class="font-semibold text-base-content">Connect the agent</p>
                <p class="text-xs text-muted">
                  Install the connector, paste the client config, and replace{' '}
                  <code class="font-mono">{AGENT_MCP_TOKEN_PLACEHOLDER}</code>.
                </p>
              </li>
            </ol>
            <details
              class="rounded-md border border-border bg-surface-alt/40 p-3"
              onToggle={(event) => setInstallerCommandsOpen(event.currentTarget.open)}
            >
              <summary class="cursor-pointer text-sm font-semibold text-base-content">
                Installer commands
              </summary>
              <Show when={installerCommandsOpen()}>
                <div class="mt-3 grid gap-3 lg:grid-cols-2">
                  <div class="space-y-2">
                    <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                      macOS / Linux
                    </h4>
                    <CopyCommandBlock
                      command={mcpInstallShellCommand}
                      onCopy={handleCopySnippet}
                      codeClass="block whitespace-pre overflow-x-auto rounded-md border border-border bg-base p-3 font-mono text-xs text-base-content"
                    />
                  </div>
                  <div class="space-y-2">
                    <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                      Windows
                    </h4>
                    <CopyCommandBlock
                      command={mcpInstallPowerShellCommand}
                      onCopy={handleCopySnippet}
                      codeClass="block whitespace-pre overflow-x-auto rounded-md border border-border bg-base p-3 font-mono text-xs text-base-content"
                    />
                  </div>
                </div>
              </Show>
            </details>
            <details
              class="rounded-md border border-border bg-surface-alt/40 p-3"
              onToggle={(event) => setClientConfigOpen(event.currentTarget.open)}
            >
              <summary class="cursor-pointer text-sm font-semibold text-base-content">
                Client config
              </summary>
              <Show when={clientConfigOpen()}>
                <div class="mt-3 space-y-4">
                  <p class="text-xs text-muted">
                    Both snippets point at this Pulse instance and use the same token placeholder:{' '}
                    <code class="font-mono">{AGENT_MCP_TOKEN_PLACEHOLDER}</code>.
                  </p>
                  <div class="space-y-2">
                    <div class="flex items-center justify-between">
                      <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                        {openCodeConfigFamily()?.label ?? 'OpenCode'}{' '}
                        <code class="font-mono normal-case">opencode.json</code>
                      </h4>
                      <Show when={copied() === openCodeMcpConfig()}>
                        <span class="text-xs text-emerald-600 dark:text-emerald-400">
                          OpenCode config copied
                        </span>
                      </Show>
                    </div>
                    <CopyCommandBlock
                      command={openCodeMcpConfig()}
                      onCopy={handleCopySnippet}
                      codeClass="block whitespace-pre overflow-x-auto rounded-md border border-border bg-base p-3 font-mono text-xs text-base-content"
                    />
                  </div>
                  <div class="space-y-2">
                    <div class="flex items-center justify-between">
                      <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                        {mcpServersConfigFamily()?.label ?? 'Claude-style clients'}{' '}
                        <code class="font-mono normal-case">mcpServers</code>
                      </h4>
                      <Show when={copied() === claudeMcpConfig()}>
                        <span class="text-xs text-emerald-600 dark:text-emerald-400">
                          mcpServers config copied
                        </span>
                      </Show>
                    </div>
                    <CopyCommandBlock
                      command={claudeMcpConfig()}
                      onCopy={handleCopySnippet}
                      codeClass="block whitespace-pre overflow-x-auto rounded-md border border-border bg-base p-3 font-mono text-xs text-base-content"
                    />
                  </div>
                </div>
              </Show>
            </details>
            <Show when={hasClientBuilderDetails()}>
              <details
                class="rounded-md border border-border bg-surface-alt/40 p-3"
                onToggle={(event) => setAdvancedClientDetailsOpen(event.currentTarget.open)}
              >
                <summary class="cursor-pointer text-sm font-semibold text-base-content">
                  Developer details
                </summary>
                <Show when={advancedClientDetailsOpen()}>
                  <div class="mt-3 space-y-3">
                    <p class="text-xs text-muted">
                      Only open this when you are building or debugging a client against this Pulse
                      instance. Normal setup only needs Patrol mode, a scoped token, the connector,
                      and the client config above.
                    </p>
                    <Show when={mcpSurfaceToolPosture()}>
                      {(posture) => (
                        <div
                          class="flex w-fit flex-wrap items-center gap-1.5 rounded-md border border-border bg-base px-2 py-1 text-[11px] font-medium text-muted"
                          title={posture().title}
                          data-testid="agent-mcp-tool-posture"
                        >
                          <span
                            class="h-1.5 w-1.5 rounded-full bg-emerald-500"
                            aria-hidden="true"
                          />
                          <span>
                            External agents expose {posture().label}{' '}
                            <span class="text-muted/80">through Patrol mode</span>
                          </span>
                        </div>
                      )}
                    </Show>
                    <Show when={surfaceContractEntries().length > 0}>
                      <details class="rounded-md border border-border bg-base p-3">
                        <summary class="cursor-pointer text-sm font-semibold text-base-content">
                          Patrol access model
                        </summary>
                        <div class="mt-3 space-y-2">
                          <p class="text-xs text-muted">
                            Built-in Pulse views and connected clients all sit behind the same
                            Patrol policy. Connected agents do not get separate powers.
                          </p>
                          <ul class="divide-y divide-border rounded-md border border-border bg-surface-alt/40">
                            <For each={surfaceContractEntries()}>
                              {(entry) => (
                                <li class="space-y-1 p-3">
                                  <div class="flex flex-wrap items-center gap-2">
                                    <span class="text-sm font-semibold text-base-content">
                                      {entry.id === AGENT_SURFACE_ID_PULSE_MCP
                                        ? 'External agents'
                                        : entry.label}
                                    </span>
                                    <For each={entry.badges}>
                                      {(badge) => (
                                        <span class="rounded-full border border-border bg-base px-2 py-0.5 text-[10px] uppercase tracking-wide text-muted">
                                          {badge}
                                        </span>
                                      )}
                                    </For>
                                  </div>
                                  <Show when={entry.description}>
                                    <p class="text-xs text-muted">{entry.description}</p>
                                  </Show>
                                </li>
                              )}
                            </For>
                          </ul>
                        </div>
                      </details>
                    </Show>
                    <Show when={hasManifestInventory()}>
                      <details class="rounded-md border border-border bg-base p-3">
                        <summary class="cursor-pointer text-sm font-semibold text-base-content">
                          Live manifest details
                        </summary>
                        <div class="mt-3 space-y-4">
                          <p class="text-xs text-muted">
                            These values come from{' '}
                            <code class="font-mono">/api/agent/capabilities</code>. Use them to
                            build clients against this instance; they do not change Patrol mode.
                          </p>
                          <Show when={workflowPrompts().length > 0}>
                            <details class="rounded-md border border-border bg-surface-alt/40 p-3">
                              <summary class="cursor-pointer text-sm font-semibold text-base-content">
                                <span>Agent starting points</span>{' '}
                                <span class="ml-2 text-xs font-normal text-muted">
                                  ({workflowPrompts().length} from manifest)
                                </span>
                              </summary>
                              <div class="mt-3 space-y-2">
                                <ul class="divide-y divide-border rounded-md border border-border bg-base">
                                  <For each={workflowPrompts()}>
                                    {(prompt) => (
                                      <li class="space-y-1 p-3">
                                        <div class="flex flex-wrap items-baseline gap-2">
                                          <span class="text-sm font-semibold text-base-content">
                                            {prompt.label ?? prompt.name}
                                          </span>
                                          <Show
                                            when={
                                              prompt.name === AGENT_WORKFLOW_PROMPT_OPERATIONS_LOOP
                                            }
                                          >
                                            <span class="rounded-full border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
                                              Patrol
                                            </span>
                                          </Show>
                                        </div>
                                        <Show when={prompt.description}>
                                          <p class="text-xs text-muted">{prompt.description}</p>
                                        </Show>
                                        <p class="text-[11px] text-muted">
                                          <span class="font-semibold">Wire name:</span>{' '}
                                          <code class="font-mono">{prompt.name}</code>
                                        </p>
                                        <Show when={(prompt.arguments?.length ?? 0) > 0}>
                                          <p class="text-[11px] text-muted">
                                            <span class="font-semibold">Arguments:</span>{' '}
                                            <For each={prompt.arguments}>
                                              {(arg, index) => (
                                                <>
                                                  <code class="font-mono">{arg.name}</code>
                                                  {arg.required ? ' required' : ' optional'}
                                                  {index() < (prompt.arguments?.length ?? 0) - 1
                                                    ? ', '
                                                    : ''}
                                                </>
                                              )}
                                            </For>
                                          </p>
                                        </Show>
                                      </li>
                                    )}
                                  </For>
                                </ul>
                              </div>
                            </details>
                          </Show>
                          <Show when={requiredScopes().length > 0}>
                            <p class="text-xs text-muted">
                              Manifest scopes:{' '}
                              <For each={requiredScopes()}>
                                {(scope, index) => (
                                  <>
                                    <code class="font-mono">{scope}</code>
                                    {index() < requiredScopes().length - 1 ? ', ' : ''}
                                  </>
                                )}
                              </For>
                              . Use narrower tokens when the client should only perform part of the
                              allowed work; each capability below shows its required scope.
                            </p>
                          </Show>
                          <Show when={errorCodeSummaries().length > 0}>
                            <details class="rounded-md border border-border bg-surface-alt/40 p-3 text-xs text-muted">
                              <summary class="cursor-pointer text-sm font-semibold text-base-content">
                                Failure codes{' '}
                                <span class="ml-2 text-xs font-normal text-muted">
                                  ({errorCodeSummaries().length} from the live manifest)
                                </span>
                              </summary>
                              <div class="mt-3 space-y-2">
                                <p>
                                  Connected agents should branch on these codes instead of scraping
                                  response text.
                                </p>
                                <div class="flex flex-wrap gap-2">
                                  <For each={errorCodeSummaries()}>
                                    {(summary) => (
                                      <span
                                        class="rounded-md border border-border bg-base px-2 py-1"
                                        title={`Declared by ${summary.capabilityNames.join(', ')}`}
                                      >
                                        <code class="font-mono">{summary.code}</code>
                                      </span>
                                    )}
                                  </For>
                                </div>
                              </div>
                            </details>
                          </Show>
                          <p class="text-xs text-muted">
                            See{' '}
                            <ExternalTextLink href={AGENT_SUBSTRATE_DOC_URL} variant="inlineSubtle">
                              docs/AGENT_SUBSTRATE.md
                            </ExternalTextLink>{' '}
                            for build instructions on <code class="font-mono">cmd/pulse-mcp</code>{' '}
                            (including the <code class="font-mono">--emit-notifications</code> flag
                            and known limitations), the companion HTTP example at{' '}
                            <code class="font-mono">cmd/agent-probe</code>, and the substrate's
                            design notes.
                          </p>
                          <Show when={manifest() && grouped().length > 0}>
                            <details class="rounded-md border border-border bg-surface-alt/40 p-3">
                              <summary class="cursor-pointer text-sm font-semibold text-base-content">
                                Agent capabilities{' '}
                                <span class="ml-2 text-xs font-normal text-muted">
                                  (manifest {manifest()?.version} -{' '}
                                  {manifest()?.capabilities.length ?? 0} total)
                                </span>
                              </summary>
                              <div class="mt-3 space-y-4">
                                <For each={grouped()}>
                                  {(section) => (
                                    <div class="space-y-2">
                                      <div>
                                        <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                                          {section.label}
                                        </h4>
                                        <Show when={section.description}>
                                          <p class="text-xs text-muted">{section.description}</p>
                                        </Show>
                                      </div>
                                      <ul class="divide-y divide-border rounded-md border border-border bg-base">
                                        <For each={section.entries}>
                                          {(cap) => (
                                            <li class="space-y-1 p-3">
                                              <div class="flex flex-wrap items-baseline gap-2">
                                                <code class="font-mono text-sm font-semibold text-base-content">
                                                  {cap.name}
                                                </code>
                                                <span class="text-xs text-muted">
                                                  <code class="font-mono">{cap.method}</code>{' '}
                                                  {cap.path}
                                                </span>
                                                <span class="ml-auto rounded-full border border-border bg-surface-alt px-2 py-0.5 text-[10px] uppercase tracking-wide text-muted">
                                                  {cap.scope}
                                                </span>
                                              </div>
                                              <p class="text-xs text-muted">{cap.description}</p>
                                              <Show
                                                when={cap.errorCodes && cap.errorCodes.length > 0}
                                              >
                                                <p class="text-[11px] text-muted">
                                                  <span class="font-semibold">
                                                    Stable error codes:
                                                  </span>{' '}
                                                  <For each={cap.errorCodes}>
                                                    {(code, idx) => (
                                                      <>
                                                        <code class="font-mono">{code}</code>
                                                        {idx() < cap.errorCodes!.length - 1
                                                          ? ', '
                                                          : ''}
                                                      </>
                                                    )}
                                                  </For>
                                                </p>
                                              </Show>
                                            </li>
                                          )}
                                        </For>
                                      </ul>
                                    </div>
                                  )}
                                </For>
                              </div>
                            </details>
                          </Show>
                        </div>
                      </details>
                    </Show>
                  </div>
                </Show>
              </details>
            </Show>
          </div>
        </Show>

        <Show when={manifest.loading}>
          <p class="text-sm text-muted">Loading capabilities…</p>
        </Show>

        <Show when={manifest.error}>
          <p class="text-sm text-red-600 dark:text-red-300">
            Could not load <code class="font-mono">/api/agent/capabilities</code>:{' '}
            {String(manifest.error)}
          </p>
        </Show>
      </div>
    </SettingsPanel>
  );
};

export default AgentIntegrationsPanel;
