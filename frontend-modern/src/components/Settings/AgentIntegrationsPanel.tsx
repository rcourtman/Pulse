import { Component, createMemo, createResource, createSignal, For, Show } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { AGENT_SUBSTRATE_DOC_URL } from '@/utils/docsLinks';
import CopyCommandBlock from './CopyCommandBlock';

// AgentCapability mirrors the wire shape of one entry in
// /api/agent/capabilities. Defined inline so this component does
// not depend on a shared API client; the manifest endpoint is
// public and stable.
interface AgentCapability {
  name: string;
  description: string;
  category: string;
  method: string;
  path: string;
  scope: string;
  responseShape?: string;
  errorCodes?: string[];
  requestBodyShape?: string;
}

interface AgentCapabilitiesManifest {
  version: string;
  capabilities: AgentCapability[];
}

// Category presentation order on screen. Matches the closed set
// the backend pin enforces (see TestAgentCapabilitiesManifest_
// CategoriesAreClosed). Capabilities with an unknown category are
// rendered last under a generic "other" heading so a future
// addition that the panel does not yet know about still renders.
const CATEGORY_ORDER: ReadonlyArray<{ id: string; label: string; description: string }> = [
  {
    id: 'context',
    label: 'Context',
    description: 'Discovery and read-only situated reads. Agents start here.',
  },
  {
    id: 'operator-state',
    label: 'Operator state',
    description: 'Per-resource intent: intentionally-offline, never-auto-remediate, maintenance windows.',
  },
  {
    id: 'finding',
    label: 'Patrol findings',
    description: 'Acknowledge, snooze, dismiss, or resolve findings the patrol runtime raised.',
  },
  {
    id: 'action',
    label: 'Action governance',
    description: 'Plan, approve, and execute capability invocations against a resource.',
  },
];

async function fetchManifest(): Promise<AgentCapabilitiesManifest> {
  const response = await fetch('/api/agent/capabilities', {
    headers: { Accept: 'application/json' },
  });
  if (!response.ok) {
    throw new Error(`manifest fetch failed: ${response.status}`);
  }
  return (await response.json()) as AgentCapabilitiesManifest;
}

// formatMcpConfig builds the JSON snippet an integrator drops into
// claude_desktop_config.json (or .mcp.json for Claude Code). The
// host is the deployment's own origin so the snippet is correct
// for whatever URL the operator is reading the panel from.
function formatMcpConfig(origin: string): string {
  const config = {
    mcpServers: {
      pulse: {
        command: 'pulse-mcp',
        args: ['--base-url', origin],
        env: {
          PULSE_API_TOKEN: '<your-api-token>',
        },
      },
    },
  };
  return JSON.stringify(config, null, 2);
}

// All three external resources (the MCP adapter README, the agent-probe HTTP
// example, the substrate design notes) are consolidated under the shipped
// AGENT_SUBSTRATE.md doc, which links out to the cmd/* source files. Avoids
// frontend-runtime links pointing at unpinned GitHub main paths.

export const AgentIntegrationsPanel: Component = () => {
  const [manifest] = createResource(fetchManifest);
  const [copied, setCopied] = createSignal<string | null>(null);
  const origin = typeof window !== 'undefined' ? window.location.origin : 'http://localhost:7655';
  const mcpConfig = formatMcpConfig(origin);

  const grouped = createMemo(() => {
    const m = manifest();
    if (!m) return [];
    // Index capabilities by category, then return them in
    // CATEGORY_ORDER. Unknown categories spill into a generic
    // "other" bucket so the panel never silently drops a
    // capability the operator should be able to see.
    const byCategory = new Map<string, AgentCapability[]>();
    for (const cap of m.capabilities) {
      const list = byCategory.get(cap.category) ?? [];
      list.push(cap);
      byCategory.set(cap.category, list);
    }
    const sections: Array<{ id: string; label: string; description?: string; entries: AgentCapability[] }> = [];
    for (const known of CATEGORY_ORDER) {
      const entries = byCategory.get(known.id);
      if (entries && entries.length > 0) {
        sections.push({ id: known.id, label: known.label, description: known.description, entries });
        byCategory.delete(known.id);
      }
    }
    for (const [unknownCategory, entries] of byCategory) {
      sections.push({ id: unknownCategory, label: unknownCategory, entries });
    }
    return sections;
  });

  const handleCopySnippet = async (snippet: string) => {
    await navigator.clipboard.writeText(snippet);
    setCopied(snippet);
    window.setTimeout(() => setCopied(null), 2000);
  };

  return (
    <SettingsPanel title="Agent integrations" noPadding>
      <div class="space-y-5 p-4 sm:p-6">
        <div class="space-y-2 text-sm text-muted">
          <p>
            Pulse exposes a manifest-driven agent surface for MCP and HTTP clients. An external
            agent (Claude Desktop, Claude Code, or a custom integration) discovers what it can do
            via <code class="font-mono text-xs">/api/agent/capabilities</code>, then calls the
            declared endpoints with an API token.
          </p>
          <p>
            The capabilities below are read live from this Pulse instance. Adding a capability on
            the backend adds it here automatically; nothing in this panel is hardcoded.
          </p>
        </div>

        <div class="space-y-3">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <h3 class="text-sm font-semibold text-base-content">Claude Desktop / Claude Code config</h3>
            <Show when={copied() === mcpConfig}>
              <span class="text-xs text-emerald-600 dark:text-emerald-400">Copied to clipboard</span>
            </Show>
          </div>
          <p class="text-xs text-muted">
            First, install <code class="font-mono">pulse-mcp</code>. The fastest path:{' '}
            <code class="font-mono">curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install-mcp.sh | bash</code>{' '}
            (or <code class="font-mono">irm .../install-mcp.ps1 | iex</code> on Windows). Then drop
            this block into{' '}
            <code class="font-mono">~/Library/Application Support/Claude/claude_desktop_config.json</code>{' '}
            (Claude Desktop) or your project's <code class="font-mono">.mcp.json</code> (Claude
            Code). Mint a token below with <code class="font-mono">monitoring:read</code> (and{' '}
            <code class="font-mono">monitoring:write</code> for the operator-state write tools), then
            replace <code class="font-mono">&lt;your-api-token&gt;</code>.
          </p>
          <CopyCommandBlock
            command={mcpConfig}
            onCopy={handleCopySnippet}
            codeClass="block whitespace-pre overflow-x-auto rounded-md border border-border bg-base p-3 font-mono text-xs text-base-content"
          />
          <p class="text-xs text-muted">
            See{' '}
            <a class="text-blue-600 hover:underline dark:text-blue-300" href={AGENT_SUBSTRATE_DOC_URL} target="_blank" rel="noreferrer">
              docs/AGENT_SUBSTRATE.md
            </a>{' '}
            for build instructions on <code class="font-mono">cmd/pulse-mcp</code>{' '}
            (including the <code class="font-mono">--emit-notifications</code> flag and known
            limitations), the companion HTTP example at <code class="font-mono">cmd/agent-probe</code>,
            and the substrate's design notes.
          </p>
        </div>

        <Show when={manifest.loading}>
          <p class="text-sm text-muted">Loading capabilities…</p>
        </Show>

        <Show when={manifest.error}>
          <p class="text-sm text-red-600 dark:text-red-300">
            Could not load <code class="font-mono">/api/agent/capabilities</code>:{' '}
            {String(manifest.error)}
          </p>
        </Show>

        <Show when={manifest() && grouped().length > 0}>
          <div class="space-y-4">
            <div class="flex items-center justify-between">
              <h3 class="text-sm font-semibold text-base-content">
                Declared capabilities (manifest {manifest()?.version})
              </h3>
              <span class="text-xs text-muted">
                {manifest()?.capabilities.length ?? 0} total
              </span>
            </div>
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
                  <ul class="divide-y divide-border rounded-md border border-border">
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
                                    {idx() < (cap.errorCodes!.length - 1) ? ', ' : ''}
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
        </Show>
      </div>
    </SettingsPanel>
  );
};

export default AgentIntegrationsPanel;
