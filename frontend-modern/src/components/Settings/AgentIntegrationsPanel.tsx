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
  AGENT_SURFACE_ID_PULSE_MCP,
  AGENT_WORKFLOW_PROMPT_OPERATIONS_LOOP,
  fetchAgentCapabilitiesManifest,
  getAgentCapabilityErrorCodeSummaries,
  getAgentWorkflowPrompts,
  getAgentManifestSurfaceToolContract,
  getAgentSurfaceToolPosturePresentation,
  getAgentSurfaceContractEntries,
  groupAgentCapabilitiesByManifestCategories,
  normalizeAgentMCPAdapter,
} from '@/api/agentCapabilities';
import { ButtonLink } from '@/components/shared/Button';
import { Card } from '@/components/shared/Card';
import { ExternalTextLink } from '@/components/shared/ExternalTextLink';
import { SelectablePillButton } from '@/components/shared/SelectablePillButton';
import { updateStore } from '@/stores/updates';
import {
  API_TOKEN_CREATE_ANCHOR,
  API_TOKEN_PRESET_QUERY_PARAM,
  EXTERNAL_AGENT_SETUP_ANCHOR,
  PATROL_CONTROL_PATH,
  PULSE_MCP_SETUP_ANCHOR,
  PULSE_MCP_TOKEN_SETUP_PATH,
  SETTINGS_API_ACCESS_PATH,
  isExternalAgentSetupHash,
} from '@/routing/resourceLinks';
import { AGENT_SUBSTRATE_DOC_URL } from '@/utils/docsLinks';
import KeyRoundIcon from 'lucide-solid/icons/key-round';
import SettingsIcon from 'lucide-solid/icons/settings';
import CopyCommandBlock from './CopyCommandBlock';
import {
  API_TOKEN_EXTERNAL_AGENT_PROPOSE_PRESET_LABEL,
  API_TOKEN_EXTERNAL_AGENT_READ_ONLY_PRESET_LABEL,
} from './apiTokenManagerModel';

type InstallMethodId = 'homebrew' | 'unix' | 'windows';
type AgentClientId = 'claude-code' | 'codex' | 'opencode' | 'claude-desktop';

interface InstallMethodOption {
  id: InstallMethodId;
  label: string;
  command: string;
  note: string;
}

interface AgentClientOption {
  id: AgentClientId;
  label: string;
  configPathHint: string;
}

const RELEASE_TAG_RE = /^v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/;

const resolveReleaseTag = (version?: string | null): string | null => {
  const trimmed = version?.trim();
  if (!trimmed || !RELEASE_TAG_RE.test(trimmed)) return null;
  return trimmed.startsWith('v') ? trimmed : `v${trimmed}`;
};

const buildInstallMethodOptions = (releaseTag: string | null): InstallMethodOption[] => [
  {
    id: 'homebrew',
    label: 'Homebrew',
    command: 'brew install rcourtman/tap/pulse',
    note: 'macOS and Linux via the signed Pulse Homebrew tap.',
  },
  {
    id: 'unix',
    label: 'macOS / Linux (verified)',
    command: releaseTag
      ? [
          `gh release download ${releaseTag} --repo rcourtman/Pulse --pattern install-cli.sh`,
          `gh release verify-asset ${releaseTag} install-cli.sh --repo rcourtman/Pulse`,
          `PULSE_CLI_VERSION=${releaseTag} bash ./install-cli.sh`,
        ].join('\n')
      : [
          'PULSE_CLI_VERSION=$(gh release view --repo rcourtman/Pulse --json tagName -q .tagName)',
          'gh release download "$PULSE_CLI_VERSION" --repo rcourtman/Pulse --pattern install-cli.sh',
          'gh release verify-asset "$PULSE_CLI_VERSION" install-cli.sh --repo rcourtman/Pulse',
          'PULSE_CLI_VERSION="$PULSE_CLI_VERSION" bash ./install-cli.sh',
        ].join('\n'),
    note: releaseTag
      ? `Downloads and verifies release ${releaseTag} before running.`
      : 'Resolves the latest published Pulse release tag and verifies the asset before running.',
  },
  {
    id: 'windows',
    label: 'Windows (verified)',
    command: releaseTag
      ? [
          `gh release download ${releaseTag} --repo rcourtman/Pulse --pattern install-cli.ps1`,
          `gh release verify-asset ${releaseTag} install-cli.ps1 --repo rcourtman/Pulse`,
          `.\\install-cli.ps1 -Version ${releaseTag}`,
        ].join('\n')
      : [
          '$PulseCliVersion = gh release view --repo rcourtman/Pulse --json tagName -q .tagName',
          'gh release download $PulseCliVersion --repo rcourtman/Pulse --pattern install-cli.ps1',
          'gh release verify-asset $PulseCliVersion install-cli.ps1 --repo rcourtman/Pulse',
          '.\\install-cli.ps1 -Version $PulseCliVersion',
        ].join('\n'),
    note: releaseTag
      ? `Downloads and verifies release ${releaseTag} before running.`
      : 'Resolves the latest published Pulse release tag and verifies the asset before running.',
  },
];

const AGENT_CLIENT_OPTIONS: AgentClientOption[] = [
  {
    id: 'claude-code',
    label: 'Claude Code',
    configPathHint: '~/.claude.json',
  },
  {
    id: 'codex',
    label: 'Codex',
    configPathHint: '~/.codex/config.toml',
  },
  {
    id: 'opencode',
    label: 'OpenCode',
    configPathHint: '~/.config/opencode/opencode.json',
  },
  {
    id: 'claude-desktop',
    label: 'Claude Desktop',
    configPathHint: 'claude_desktop_config.json',
  },
];

const formatTOMLKey = (key: string): string =>
  /^[A-Za-z0-9_-]+$/.test(key) ? key : JSON.stringify(key);

const formatPulseCLIMCPServersSnippet = (serverName: string): string =>
  JSON.stringify(
    {
      mcpServers: {
        [serverName]: {
          command: 'pulse',
          args: ['mcp'],
        },
      },
    },
    null,
    2,
  );

const formatPulseCLICodexTOMLSnippet = (serverName: string): string =>
  `[mcp_servers.${formatTOMLKey(serverName)}]\ncommand = "pulse"\nargs = ["mcp"]`;

const formatPulseCLIOpenCodeSnippet = (serverName: string): string =>
  JSON.stringify(
    {
      $schema: 'https://opencode.ai/config.json',
      mcp: {
        [serverName]: {
          type: 'local',
          command: ['pulse', 'mcp'],
          enabled: true,
        },
      },
    },
    null,
    2,
  );

const PROPOSE_TOKEN_SETUP_PATH = `${SETTINGS_API_ACCESS_PATH}?${API_TOKEN_PRESET_QUERY_PARAM}=external-agent-propose#${API_TOKEN_CREATE_ANCHOR}`;

const COMMAND_BLOCK_CONTAINER_CLASS =
  'flex items-start gap-2 rounded-md border border-border bg-page p-2 pl-3';
const CODE_BLOCK_CLASS =
  'min-w-0 flex-1 whitespace-pre overflow-x-auto py-1.5 font-mono text-xs text-base-content';

export const AgentIntegrationsPanel: Component = () => {
  const [manifestState, { refetch: refetchManifest }] = createResource(async () => {
    try {
      const data = await fetchAgentCapabilitiesManifest();
      return { data, error: null as string | null };
    } catch (err) {
      return {
        data: null,
        error: err instanceof Error && err.message ? err.message : String(err),
      };
    }
  });
  const manifest = () => manifestState()?.data ?? undefined;
  const manifestError = () => manifestState()?.error ?? null;
  const [copied, setCopied] = createSignal<string | null>(null);
  const [routeHash, setRouteHash] = createSignal('');
  const [panelHighlight, setPanelHighlight] = createSignal(false);
  const [installMethod, setInstallMethod] = createSignal<InstallMethodId>('homebrew');
  const [selectedClient, setSelectedClient] = createSignal<AgentClientId>('claude-code');
  const [manualConfigOpen, setManualConfigOpen] = createSignal(false);
  const [advancedClientDetailsOpen, setAdvancedClientDetailsOpen] = createSignal(false);
  const origin = typeof window !== 'undefined' ? window.location.origin : 'http://localhost:7655';

  let highlightTimer: number | undefined;
  let routeFocusTimer: number | undefined;

  const installMethodOptions = createMemo(() =>
    buildInstallMethodOptions(resolveReleaseTag(updateStore.versionInfo()?.version)),
  );
  const activeInstallMethod = createMemo(
    () =>
      installMethodOptions().find((option) => option.id === installMethod()) ??
      installMethodOptions()[0],
  );
  const activeAgentClient = createMemo(
    () =>
      AGENT_CLIENT_OPTIONS.find((option) => option.id === selectedClient()) ??
      AGENT_CLIENT_OPTIONS[0],
  );

  const authLoginCommand = createMemo(() => `pulse auth login ${origin}`);
  const connectAndVerifyCommand = createMemo(
    () => `pulse connect ${activeAgentClient().id}\npulse doctor`,
  );

  const mcpAdapter = createMemo(() => normalizeAgentMCPAdapter(manifest()?.mcpAdapter));
  const serverName = createMemo(() => mcpAdapter().serverName || 'pulse');
  const claudeManualConfig = createMemo(() => formatPulseCLIMCPServersSnippet(serverName()));
  const codexManualConfig = createMemo(() => formatPulseCLICodexTOMLSnippet(serverName()));
  const openCodeManualConfig = createMemo(() => formatPulseCLIOpenCodeSnippet(serverName()));

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
    <Card
      id={EXTERNAL_AGENT_SETUP_ANCHOR}
      data-settings-panel
      padding="none"
      border={false}
      class={`overflow-hidden border border-border scroll-mt-20 transition-shadow ${
        panelHighlight() ? 'ring-2 ring-blue-500 shadow-xs' : ''
      }`}
    >
      <span id={PULSE_MCP_SETUP_ANCHOR} class="sr-only" aria-hidden="true" />
      <div class="space-y-5 p-4 sm:p-6">
        <div class="max-w-3xl space-y-1.5 text-sm text-muted">
          <p>
            Connect Claude Code, Codex, OpenCode, or Claude Desktop to Pulse with the{' '}
            <code class="font-mono text-base-content">pulse</code> CLI. Starts read-only with your
            token stored in your OS keychain, never in client config files.
          </p>
        </div>

        <div id="external-agent-setup-details" class="space-y-4">
          <ol class="divide-y divide-border rounded-md border border-border text-sm">
            <li class="space-y-3 p-4">
              <div class="flex flex-col gap-2.5">
                <div class="space-y-0.5">
                  <p class="text-[11px] font-semibold uppercase tracking-wide text-muted">Step 1</p>
                  <h3 class="font-semibold text-base-content">
                    Install the <code class="font-mono">pulse</code> CLI
                  </h3>
                  <p class="text-xs text-muted">
                    Install on the workstation running your AI agent (not on monitored hosts).
                  </p>
                </div>
                <div class="flex flex-wrap gap-1.5" role="group" aria-label="CLI install method">
                  <For each={installMethodOptions()}>
                    {(option) => (
                      <SelectablePillButton
                        size="md"
                        active={installMethod() === option.id}
                        onClick={() => setInstallMethod(option.id)}
                      >
                        {option.label}
                      </SelectablePillButton>
                    )}
                  </For>
                </div>
              </div>
              <CopyCommandBlock
                command={activeInstallMethod().command}
                onCopy={handleCopySnippet}
                buttonLayout="inline"
                containerClass={COMMAND_BLOCK_CONTAINER_CLASS}
                codeClass={CODE_BLOCK_CLASS}
              />
              <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted">
                <p>
                  {activeInstallMethod().note}{' '}
                  <ExternalTextLink href={AGENT_SUBSTRATE_DOC_URL} variant="inlineSubtle">
                    Full verification &amp; build instructions
                  </ExternalTextLink>
                </p>
                <Show when={copied() === activeInstallMethod().command}>
                  <span role="status" class="font-medium text-emerald-600 dark:text-emerald-400">
                    Copied
                  </span>
                </Show>
              </div>
            </li>

            <li class="space-y-3 p-4">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="space-y-0.5">
                  <p class="text-[11px] font-semibold uppercase tracking-wide text-muted">Step 2</p>
                  <h3 class="font-semibold text-base-content">Sign in read-only</h3>
                  <p class="text-xs text-muted">
                    Uses a read-only <code class="font-mono">monitoring:read</code> token by default
                    and stores it in your OS keychain (with a <code class="font-mono">0600</code>{' '}
                    fallback on headless hosts).
                  </p>
                </div>
                <ButtonLink
                  href={PULSE_MCP_TOKEN_SETUP_PATH}
                  variant="secondary"
                  size="settingsAction"
                  class="w-fit shrink-0 gap-2 font-semibold"
                >
                  <KeyRoundIcon class="h-4 w-4" aria-hidden="true" />
                  Create token
                </ButtonLink>
              </div>
              <CopyCommandBlock
                command={authLoginCommand()}
                onCopy={handleCopySnippet}
                buttonLayout="inline"
                containerClass={COMMAND_BLOCK_CONTAINER_CLASS}
                codeClass={CODE_BLOCK_CLASS}
              />
              <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted">
                <p>
                  Select the{' '}
                  <span class="font-medium text-base-content">
                    {API_TOKEN_EXTERNAL_AGENT_READ_ONLY_PRESET_LABEL}
                  </span>{' '}
                  preset when creating your token.
                </p>
                <Show when={copied() === authLoginCommand()}>
                  <span role="status" class="font-medium text-emerald-600 dark:text-emerald-400">
                    Copied
                  </span>
                </Show>
              </div>
            </li>

            <li class="space-y-3 p-4">
              <div class="flex flex-col gap-2.5">
                <div class="space-y-0.5">
                  <p class="text-[11px] font-semibold uppercase tracking-wide text-muted">Step 3</p>
                  <h3 class="font-semibold text-base-content">Connect your agent and verify</h3>
                  <p class="text-xs text-muted">
                    Writes your user-local config (never a repo{' '}
                    <code class="font-mono">.mcp.json</code>) and checks connectivity.
                  </p>
                </div>
                <div class="flex flex-wrap gap-1.5" role="group" aria-label="External agent client">
                  <For each={AGENT_CLIENT_OPTIONS}>
                    {(client) => (
                      <SelectablePillButton
                        size="md"
                        active={selectedClient() === client.id}
                        onClick={() => setSelectedClient(client.id)}
                      >
                        {client.label}
                      </SelectablePillButton>
                    )}
                  </For>
                </div>
              </div>
              <CopyCommandBlock
                command={connectAndVerifyCommand()}
                onCopy={handleCopySnippet}
                buttonLayout="inline"
                containerClass={COMMAND_BLOCK_CONTAINER_CLASS}
                codeClass={CODE_BLOCK_CLASS}
              />
              <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted">
                <p>
                  Configures{' '}
                  <span class="font-medium text-base-content">{activeAgentClient().label}</span> at{' '}
                  <code class="font-mono">{activeAgentClient().configPathHint}</code> with zero
                  embedded tokens. Ask your agent{' '}
                  <span class="italic text-base-content">
                    &ldquo;What happened overnight?&rdquo;
                  </span>{' '}
                  or run <code class="font-mono">pulse changes --since 12h</code>.
                </p>
                <Show when={copied() === connectAndVerifyCommand()}>
                  <span role="status" class="font-medium text-emerald-600 dark:text-emerald-400">
                    Copied
                  </span>
                </Show>
              </div>
            </li>
          </ol>

          <section
            aria-labelledby="external-agent-propose-heading"
            class="rounded-md border border-border bg-surface-alt p-4"
          >
            <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div class="max-w-2xl space-y-1">
                <h3
                  id="external-agent-propose-heading"
                  class="text-sm font-semibold text-base-content"
                >
                  Want agents to propose fixes?
                </h3>
                <p class="text-xs text-muted">
                  External agents start read-only. To let an agent propose governed fixes to the
                  Decision Inbox (without self-approving or executing), create a token with the{' '}
                  <span class="font-medium text-base-content">
                    {API_TOKEN_EXTERNAL_AGENT_PROPOSE_PRESET_LABEL}
                  </span>{' '}
                  preset (<code class="font-mono">monitoring:read</code> +{' '}
                  <code class="font-mono">actions:plan</code>) and review your Patrol mode.
                </p>
              </div>
              <div class="flex shrink-0 flex-wrap gap-2">
                <ButtonLink
                  href={PROPOSE_TOKEN_SETUP_PATH}
                  variant="secondary"
                  size="settingsAction"
                  class="w-fit gap-2 font-semibold"
                >
                  <KeyRoundIcon class="h-4 w-4" aria-hidden="true" />
                  Create propose token
                </ButtonLink>
                <ButtonLink
                  href={PATROL_CONTROL_PATH}
                  variant="secondary"
                  size="settingsAction"
                  class="w-fit gap-2 font-semibold"
                >
                  <SettingsIcon class="h-4 w-4" aria-hidden="true" />
                  Choose Patrol mode
                </ButtonLink>
              </div>
            </div>
          </section>

          <details
            class="rounded-md border border-border p-3"
            onToggle={(event) => setManualConfigOpen(event.currentTarget.open)}
          >
            <summary class="cursor-pointer text-sm font-semibold text-base-content">
              Manual config
            </summary>
            <Show when={manualConfigOpen()}>
              <div class="mt-3 space-y-4">
                <p class="text-xs text-muted">
                  Prefer editing client configs by hand after{' '}
                  <code class="font-mono">pulse auth login</code>? Every client launches{' '}
                  <code class="font-mono">pulse mcp</code> and reads credentials from your OS
                  keychain. Never paste API tokens into config files.
                </p>

                <div class="space-y-2">
                  <div class="flex items-center justify-between gap-2">
                    <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                      Claude Code (<code class="font-mono normal-case">~/.claude.json</code>) &amp;
                      Claude Desktop (
                      <code class="font-mono normal-case">claude_desktop_config.json</code>)
                    </h4>
                    <Show when={copied() === claudeManualConfig()}>
                      <span class="text-xs text-emerald-600 dark:text-emerald-400">
                        mcpServers config copied
                      </span>
                    </Show>
                  </div>
                  <CopyCommandBlock
                    command={claudeManualConfig()}
                    onCopy={handleCopySnippet}
                    buttonLayout="inline"
                    containerClass={COMMAND_BLOCK_CONTAINER_CLASS}
                    codeClass={CODE_BLOCK_CLASS}
                  />
                </div>

                <div class="space-y-2">
                  <div class="flex items-center justify-between gap-2">
                    <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                      Codex <code class="font-mono normal-case">~/.codex/config.toml</code>
                    </h4>
                    <Show when={copied() === codexManualConfig()}>
                      <span class="text-xs text-emerald-600 dark:text-emerald-400">
                        Codex config copied
                      </span>
                    </Show>
                  </div>
                  <CopyCommandBlock
                    command={codexManualConfig()}
                    onCopy={handleCopySnippet}
                    buttonLayout="inline"
                    containerClass={COMMAND_BLOCK_CONTAINER_CLASS}
                    codeClass={CODE_BLOCK_CLASS}
                  />
                </div>

                <div class="space-y-2">
                  <div class="flex items-center justify-between gap-2">
                    <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                      OpenCode{' '}
                      <code class="font-mono normal-case">~/.config/opencode/opencode.json</code>
                    </h4>
                    <Show when={copied() === openCodeManualConfig()}>
                      <span class="text-xs text-emerald-600 dark:text-emerald-400">
                        OpenCode config copied
                      </span>
                    </Show>
                  </div>
                  <CopyCommandBlock
                    command={openCodeManualConfig()}
                    onCopy={handleCopySnippet}
                    buttonLayout="inline"
                    containerClass={COMMAND_BLOCK_CONTAINER_CLASS}
                    codeClass={CODE_BLOCK_CLASS}
                  />
                </div>
              </div>
            </Show>
          </details>

          <Show
            when={manifestState.loading || Boolean(manifestError()) || hasClientBuilderDetails()}
          >
            <details
              class="rounded-md border border-border p-3"
              onToggle={(event) => setAdvancedClientDetailsOpen(event.currentTarget.open)}
            >
              <summary class="cursor-pointer text-sm font-semibold text-base-content">
                Developer details
              </summary>
              <Show when={advancedClientDetailsOpen()}>
                <div class="mt-3 space-y-3">
                  <p class="text-xs text-muted">
                    Only open this when you are building or debugging a client against this Pulse
                    instance. Normal setup only needs the three CLI steps above.
                  </p>
                  <Show when={manifestState.loading}>
                    <p class="text-xs text-muted" role="status">
                      Loading capabilities...
                    </p>
                  </Show>
                  <Show when={manifestError()}>
                    {(errorMessage) => (
                      <div
                        role="alert"
                        class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700 dark:border-red-800 dark:bg-red-900/25 dark:text-red-300"
                      >
                        <span>
                          Could not load <code class="font-mono">/api/agent/capabilities</code>:{' '}
                          {errorMessage()}
                        </span>
                        <button
                          type="button"
                          onClick={() => void refetchManifest()}
                          class="shrink-0 rounded-sm border border-red-300 px-2.5 py-1 font-medium hover:bg-red-100 dark:border-red-700 dark:hover:bg-red-900/40"
                        >
                          Retry
                        </button>
                      </div>
                    )}
                  </Show>
                  <Show when={mcpSurfaceToolPosture()}>
                    {(posture) => (
                      <div
                        class="flex w-fit flex-wrap items-center gap-1.5 rounded-md border border-border bg-page px-2 py-1 text-[11px] font-medium text-muted"
                        title={posture().title}
                        data-testid="agent-mcp-tool-posture"
                      >
                        <span class="h-1.5 w-1.5 rounded-full bg-emerald-500" aria-hidden="true" />
                        <span>External agents expose {posture().label} through Patrol mode</span>
                      </div>
                    )}
                  </Show>
                  <Show when={surfaceContractEntries().length > 0}>
                    <details class="rounded-md border border-border bg-page p-3">
                      <summary class="cursor-pointer text-sm font-semibold text-base-content">
                        Patrol access model
                      </summary>
                      <div class="mt-3 space-y-2">
                        <p class="text-xs text-muted">
                          Built-in Pulse views and connected clients all sit behind the same Patrol
                          policy. Connected agents do not get separate powers.
                        </p>
                        <ul class="divide-y divide-border rounded-md border border-border">
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
                                      <span class="rounded-full border border-border bg-page px-2 py-0.5 text-[10px] uppercase tracking-wide text-muted">
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
                    <div class="space-y-3 rounded-md border border-border bg-page p-3">
                      <div class="space-y-1">
                        <h4 class="text-sm font-semibold text-base-content">
                          Live manifest details
                        </h4>
                        <p class="text-xs text-muted">
                          These values come from{' '}
                          <code class="font-mono">/api/agent/capabilities</code>. Use them to build
                          clients against this instance. They do not change Patrol mode.
                        </p>
                      </div>
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
                          allowed work. Each capability below shows its required scope.
                        </p>
                      </Show>
                      <Show when={workflowPrompts().length > 0}>
                        <details class="rounded-md border border-border bg-surface p-3">
                          <summary class="cursor-pointer text-sm font-semibold text-base-content">
                            <span>Agent starting points</span>{' '}
                            <span class="ml-2 text-xs font-normal text-muted">
                              ({workflowPrompts().length} from manifest)
                            </span>
                          </summary>
                          <div class="mt-3 space-y-2">
                            <ul class="divide-y divide-border rounded-md border border-border bg-page">
                              <For each={workflowPrompts()}>
                                {(prompt) => (
                                  <li class="space-y-1 p-3">
                                    <div class="flex flex-wrap items-baseline gap-2">
                                      <span class="text-sm font-semibold text-base-content">
                                        {prompt.label ?? prompt.name}
                                      </span>
                                      <Show
                                        when={prompt.name === AGENT_WORKFLOW_PROMPT_OPERATIONS_LOOP}
                                      >
                                        <span class="rounded-full border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/25 dark:text-emerald-300">
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
                      <Show when={errorCodeSummaries().length > 0}>
                        <details class="rounded-md border border-border bg-surface p-3 text-xs text-muted">
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
                                    class="rounded-md border border-border bg-page px-2 py-1"
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
                      <Show when={manifest() && grouped().length > 0}>
                        <details class="rounded-md border border-border bg-surface p-3">
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
                                  <ul class="divide-y divide-border rounded-md border border-border bg-page">
                                    <For each={section.entries}>
                                      {(cap) => (
                                        <li class="space-y-1 p-3">
                                          <div class="flex flex-wrap items-baseline gap-2">
                                            <code class="font-mono text-sm font-semibold text-base-content">
                                              {cap.name}
                                            </code>
                                            <span class="text-xs text-muted">
                                              <code class="font-mono">{cap.method}</code> {cap.path}
                                            </span>
                                            <span class="ml-auto rounded-full border border-border bg-surface-alt px-2 py-0.5 text-[10px] uppercase tracking-wide text-muted">
                                              {cap.scope}
                                            </span>
                                          </div>
                                          <p class="text-xs text-muted">{cap.description}</p>
                                          <Show when={cap.errorCodes && cap.errorCodes.length > 0}>
                                            <p class="text-[11px] text-muted">
                                              <span class="font-semibold">Stable error codes:</span>{' '}
                                              <For each={cap.errorCodes}>
                                                {(code, idx) => (
                                                  <>
                                                    <code class="font-mono">{code}</code>
                                                    {idx() < cap.errorCodes!.length - 1 ? ', ' : ''}
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
                      <p class="text-xs text-muted">
                        See the{' '}
                        <ExternalTextLink href={AGENT_SUBSTRATE_DOC_URL} variant="inlineSubtle">
                          External Agent Guide
                        </ExternalTextLink>{' '}
                        for CLI verification details and custom client integration notes.
                      </p>
                    </div>
                  </Show>
                </div>
              </Show>
            </details>
          </Show>
        </div>
      </div>
    </Card>
  );
};

export default AgentIntegrationsPanel;
