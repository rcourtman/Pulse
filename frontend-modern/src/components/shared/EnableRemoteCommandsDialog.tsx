import {
  Show,
  createEffect,
  createMemo,
  createSignal,
  createUniqueId,
  onCleanup,
  type Component,
} from 'solid-js';
import CheckIcon from 'lucide-solid/icons/check';
import CheckCircle2Icon from 'lucide-solid/icons/check-circle-2';
import CodeIcon from 'lucide-solid/icons/code';
import CopyIcon from 'lucide-solid/icons/copy';
import EyeIcon from 'lucide-solid/icons/eye';
import EyeOffIcon from 'lucide-solid/icons/eye-off';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import Loader2Icon from 'lucide-solid/icons/loader-2';
import TerminalIcon from 'lucide-solid/icons/terminal';
import XIcon from 'lucide-solid/icons/x';
import { NodesAPI } from '@/api/nodes';
import { ResourceAPI } from '@/api/resources';
import { SecurityAPI } from '@/api/security';
import {
  buildInfrastructureAgentDoctorPath,
  buildInfrastructureCommandAccessPath,
} from '@/components/Settings/infrastructureWorkspaceModel';
import { Button } from '@/components/shared/Button';
import { getButtonClass } from '@/components/shared/buttonModel';
import { Dialog } from '@/components/shared/Dialog';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { notificationStore } from '@/stores/notifications';
import type { SecurityStatus } from '@/types/config';
import type { Resource } from '@/types/resource';
import { copyToClipboard } from '@/utils/clipboard';
import { getPulseBaseUrl } from '@/utils/url';
import {
  buildEnableCommandsTokenName,
  buildEnableRemoteCommandsScript,
  extractHostCommandAccessContext,
  getEnableCommandsContinueLabel,
  getEnableCommandsIntroCopy,
  getEnableCommandsTerminalInstruction,
  isLeastPrivilegeAgentTarget,
  resolveCommandAccessInstallerStep,
  resolveTargetCommandPlatform,
  type EnableRemoteCommandsTarget,
} from './enableRemoteCommandsModel';

export interface EnableRemoteCommandsDialogProps {
  isOpen: boolean;
  target: EnableRemoteCommandsTarget | null;
  commandsEnabled?: boolean;
  onClose: () => void;
  onContinue?: () => void;
  onEnabled?: () => void;
  returnFocus?: () => HTMLElement | null | undefined;
}

const HOST_RECONNECT_POLL_MS = 3000;

// Retain a freshly minted command token in memory for this tab session so an
// accidental click outside the dialog while switching to a terminal window does
// not force the operator to mint a duplicate API token. Cleared as soon as the
// host reconnects with commands enabled (or on page reload).
const inFlightEnableTokensByHost = new Map<string, string>();

export function clearInFlightEnableTokensForTests(): void {
  inFlightEnableTokensByHost.clear();
}

const tokenCacheKeyForTarget = (target: EnableRemoteCommandsTarget | null): string => {
  if (!target) return '';
  const id = (target.hostResourceId || target.agentId || target.hostName).trim().toLowerCase();
  return id.startsWith('agent:') ? id.slice('agent:'.length) : id;
};

export const EnableRemoteCommandsDialog: Component<EnableRemoteCommandsDialogProps> = (props) => {
  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatus | null>(null);
  const [statusLoading, setStatusLoading] = createSignal(false);
  const [generating, setGenerating] = createSignal(false);
  const [generatedToken, setGeneratedToken] = createSignal<string | null>(null);
  const [showCommand, setShowCommand] = createSignal(false);
  const [showToken, setShowToken] = createSignal(false);
  const [commandCopied, setCommandCopied] = createSignal(false);
  const [tokenCopied, setTokenCopied] = createSignal(false);
  const [insecureMode, setInsecureMode] = createSignal(false);
  const [customCaPath, setCustomCaPath] = createSignal('');
  const [error, setError] = createSignal('');
  const [polledCommandsEnabled, setPolledCommandsEnabled] = createSignal(false);
  const [resolvedHost, setResolvedHost] = createSignal<Resource | null>(null);

  const uniqueId = createUniqueId();
  const titleId = `enable-remote-commands-dialog-title-${uniqueId}`;
  const caPathInputId = `enable-remote-commands-ca-path-${uniqueId}`;
  let notifiedEnabledForSession = false;

  const hostKey = createMemo(() => {
    const target = props.target;
    if (!target) return '';
    return `${target.hostResourceId ?? ''}|${target.agentId ?? ''}|${target.hostName}`;
  });

  const effectiveTarget = createMemo<EnableRemoteCommandsTarget | null>(() => {
    const base = props.target;
    if (!base) return null;
    const host = resolvedHost();
    if (!host) return base;
    const extracted = extractHostCommandAccessContext(host, {
      hostName: base.hostName,
      agentId: base.agentId,
      hostCommandsEnabled: base.commandsEnabled,
    });
    return {
      ...base,
      hostResourceId: base.hostResourceId || extracted.hostResourceId,
      hostName: base.hostName && base.hostName !== 'this host' ? base.hostName : extracted.hostName,
      agentId: base.agentId || extracted.agentId,
      hostname: base.hostname || extracted.hostname,
      platform: base.platform || extracted.platform,
      hostProfile: base.hostProfile || extracted.hostProfile,
      collectionMode: base.collectionMode || extracted.collectionMode,
      commandsEnabled: base.commandsEnabled ?? extracted.commandsEnabled,
    };
  });

  // Reset per-session UI state when closing or switching hosts, while restoring
  // any in-flight token minted earlier in this tab for the same host.
  createEffect(() => {
    const open = props.isOpen;
    hostKey();
    notifiedEnabledForSession = false;
    setShowCommand(false);
    setShowToken(false);
    setCommandCopied(false);
    setTokenCopied(false);
    setInsecureMode(false);
    setCustomCaPath('');
    setError('');
    setGenerating(false);
    setPolledCommandsEnabled(false);
    setResolvedHost(null);

    if (!open) {
      setGeneratedToken(null);
      return;
    }

    const target = props.target;
    const cacheKey = tokenCacheKeyForTarget(target);
    setGeneratedToken(cacheKey ? (inFlightEnableTokensByHost.get(cacheKey) ?? null) : null);

    let cancelled = false;
    setStatusLoading(true);
    void SecurityAPI.getStatus()
      .then((status) => {
        if (!cancelled) setSecurityStatus(status);
      })
      .catch(() => {
        if (!cancelled) setSecurityStatus(null);
      })
      .finally(() => {
        if (!cancelled) setStatusLoading(false);
      });

    // Callers with partial host metadata (such as the Workloads table) omit
    // platform/profile/hostname; hydrate once from the backing host resource.
    const lookupId =
      target?.hostResourceId?.trim() ||
      (target?.agentId?.trim()
        ? target.agentId.trim().startsWith('agent:')
          ? target.agentId.trim()
          : `agent:${target.agentId.trim()}`
        : '');
    const needsHostHydration =
      Boolean(lookupId) && (!target?.platform || !target?.hostname || !target?.hostProfile);
    if (needsHostHydration) {
      void ResourceAPI.getResource(lookupId)
        .then((resource) => {
          if (!cancelled && resource) {
            setResolvedHost(resource);
            if (resource.agent?.commandsEnabled === true) {
              setPolledCommandsEnabled(true);
            }
          }
        })
        .catch(() => {
          // Keep the caller's fallback metadata when the lookup fails.
        });
    }

    onCleanup(() => {
      cancelled = true;
    });
  });

  const isEnabled = createMemo(
    () =>
      props.commandsEnabled === true ||
      effectiveTarget()?.commandsEnabled === true ||
      polledCommandsEnabled(),
  );

  createEffect(() => {
    if (props.isOpen && isEnabled()) {
      const cacheKey = tokenCacheKeyForTarget(effectiveTarget());
      if (cacheKey) inFlightEnableTokensByHost.delete(cacheKey);
      if (!notifiedEnabledForSession) {
        notifiedEnabledForSession = true;
        props.onEnabled?.();
      }
    }
  });

  const requiresToken = createMemo(() => {
    const status = securityStatus();
    if (!status) return true;
    return status.requiresAuth || status.apiTokenConfigured;
  });

  const baseUrl = createMemo(() => securityStatus()?.agentUrl || getPulseBaseUrl());
  const isHttps = createMemo(() => baseUrl().trim().toLowerCase().startsWith('https://'));
  const platform = createMemo(() => resolveTargetCommandPlatform(effectiveTarget()));
  const isLeastPrivilege = createMemo(() => isLeastPrivilegeAgentTarget(effectiveTarget()));

  const commandReady = createMemo(() => {
    if (isLeastPrivilege() || isEnabled()) return false;
    if (statusLoading()) return false;
    return !requiresToken() || Boolean(generatedToken());
  });

  const commandScript = createMemo(() => {
    const target = effectiveTarget();
    if (!target || !commandReady()) return '';
    try {
      return buildEnableRemoteCommandsScript({
        baseUrl: baseUrl(),
        target,
        token: generatedToken(),
        insecure: isHttps() && insecureMode(),
        caCertPath: isHttps() ? customCaPath() : undefined,
      });
    } catch {
      return '';
    }
  });

  // Poll the backing host resource while waiting for the operator to run the
  // command, so standalone drawers and Settings Manage still flip to ready.
  createEffect(() => {
    const open = props.isOpen;
    const ready = commandReady();
    const enabled = isEnabled();
    const resourceId = effectiveTarget()?.hostResourceId?.trim();
    if (!open || !ready || enabled || !resourceId) return;

    let cancelled = false;
    const checkHost = () => {
      void ResourceAPI.getResource(resourceId)
        .then((resource) => {
          if (!cancelled && resource?.agent?.commandsEnabled === true) {
            setPolledCommandsEnabled(true);
          }
        })
        .catch(() => {
          // Ignore transient polling failures while the agent restarts.
        });
    };

    const timer = window.setInterval(checkHost, HOST_RECONNECT_POLL_MS);
    onCleanup(() => {
      cancelled = true;
      window.clearInterval(timer);
    });
  });

  const handleGenerateCommand = async () => {
    const target = effectiveTarget();
    if (!target || generating()) return;
    setGenerating(true);
    setError('');
    try {
      const response = await NodesAPI.createHostAgentInstallToken({
        enableCommands: true,
        name: buildEnableCommandsTokenName(target.hostName),
      });
      const cacheKey = tokenCacheKeyForTarget(target);
      if (cacheKey) inFlightEnableTokensByHost.set(cacheKey, response.token);
      setGeneratedToken(response.token);
      setShowCommand(false);
      setShowToken(false);
      setCommandCopied(false);
      setTokenCopied(false);
    } catch (err) {
      const message =
        err instanceof Error && err.message.trim()
          ? err.message.trim()
          : 'Could not generate a command-capable token.';
      setError(message);
    } finally {
      setGenerating(false);
    }
  };

  const handleCopyCommand = async () => {
    const script = commandScript();
    if (!script) return;
    const copied = await copyToClipboard(script);
    if (copied) {
      setCommandCopied(true);
      notificationStore.success('Command copied to clipboard.');
      window.setTimeout(() => setCommandCopied(false), 2000);
    } else {
      notificationStore.error('Could not copy command. Open View command to copy it manually.');
    }
  };

  const handleCopyToken = async () => {
    const token = generatedToken();
    if (!token) return;
    const copied = await copyToClipboard(token);
    if (copied) {
      setTokenCopied(true);
      notificationStore.success('Token copied to clipboard.');
      window.setTimeout(() => setTokenCopied(false), 2000);
    } else {
      notificationStore.error('Could not copy token. Reveal and copy it manually.');
    }
  };

  const installerHref = createMemo(() =>
    buildInfrastructureCommandAccessPath(resolveCommandAccessInstallerStep(effectiveTarget())),
  );
  const agentDoctorHref = createMemo(() =>
    buildInfrastructureAgentDoctorPath(
      effectiveTarget()?.agentId ? [effectiveTarget()!.agentId!] : [],
    ),
  );

  return (
    <Dialog
      isOpen={props.isOpen && Boolean(effectiveTarget())}
      onClose={props.onClose}
      ariaLabelledBy={titleId}
      panelClass="max-w-xl"
      returnFocus={props.returnFocus}
    >
      <Show when={effectiveTarget()}>
        {(target) => (
          <div class="flex min-h-0 flex-1 flex-col" data-testid="enable-remote-commands-dialog">
            <div class="flex shrink-0 items-start justify-between gap-3 border-b border-border px-5 py-4">
              <div class="flex items-start gap-2.5">
                <span class="mt-0.5 inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-200">
                  <TerminalIcon class="h-4 w-4" aria-hidden="true" />
                </span>
                <div>
                  <h2 id={titleId} class="text-base font-semibold text-base-content">
                    Enable remote commands on {target().hostName}
                  </h2>
                  <p class="mt-0.5 text-xs text-muted">{getEnableCommandsIntroCopy(target())}</p>
                </div>
              </div>
              <button
                type="button"
                class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500"
                aria-label="Close"
                onClick={props.onClose}
              >
                <XIcon class="h-4 w-4" aria-hidden="true" />
              </button>
            </div>

            <div class="min-h-0 flex-1 space-y-4 overflow-y-auto px-5 py-4">
              <Show
                when={!isEnabled()}
                fallback={
                  <div
                    class="flex items-start gap-3 rounded-lg border border-emerald-200 bg-emerald-50 p-3.5 text-emerald-900 dark:border-emerald-800/80 dark:bg-emerald-950/35 dark:text-emerald-100"
                    role="status"
                    data-testid="enable-remote-commands-complete"
                  >
                    <CheckCircle2Icon
                      class="mt-0.5 h-5 w-5 shrink-0 text-emerald-600 dark:text-emerald-400"
                      aria-hidden="true"
                    />
                    <div class="space-y-1 text-sm">
                      <p class="font-semibold">
                        Remote commands are enabled on {target().hostName}.
                      </p>
                      <p class="text-xs opacity-90">
                        Pulse can now apply container updates and lifecycle actions on this host.
                      </p>
                    </div>
                  </div>
                }
              >
                <Show
                  when={!isLeastPrivilege()}
                  fallback={
                    <InlineNotice tone="info">
                      <p class="font-medium">
                        {target().hostName} uses the least-privilege agent profile.
                      </p>
                      <p class="mt-1">
                        Container actions on least-privilege hosts require a separate action-runner
                        credential. Open Agent Doctor to issue and install it.
                      </p>
                    </InlineNotice>
                  }
                >
                  <Show when={error()}>
                    <InlineNotice tone="danger" role="alert">
                      {error()}
                    </InlineNotice>
                  </Show>

                  <Show
                    when={!statusLoading()}
                    fallback={
                      <div class="flex items-center gap-2 py-3 text-xs text-muted">
                        <Loader2Icon class="h-4 w-4 animate-spin" aria-hidden="true" />
                        <span>Checking Pulse server configuration...</span>
                      </div>
                    }
                  >
                    <Show
                      when={commandReady()}
                      fallback={
                        <div class="space-y-3">
                          <div class="rounded-lg border border-border bg-surface-alt/60 p-3.5 text-xs leading-5 text-muted">
                            <p class="font-medium text-base-content">What this does</p>
                            <p class="mt-1">
                              Creates a command-capable API token for {target().hostName} (root
                              access on this host, revocable anytime in{' '}
                              <span class="font-medium text-base-content">
                                Settings → Security → API Access
                              </span>
                              ) and gives you the one-time command to run on {target().hostName}.
                            </p>
                          </div>
                          <div>
                            <Button
                              type="button"
                              variant="primary"
                              size="sm"
                              isLoading={generating()}
                              onClick={() => void handleGenerateCommand()}
                            >
                              Generate command for {target().hostName}
                            </Button>
                          </div>
                        </div>
                      }
                    >
                      <div class="space-y-3" data-testid="enable-remote-commands-ready">
                        <div class="space-y-2 rounded-lg border border-border bg-surface-alt/60 p-3">
                          <div class="flex flex-wrap items-center justify-between gap-2">
                            <p class="text-xs font-medium text-base-content">
                              {generatedToken() && platform() !== 'windows' ? '1. ' : ''}
                              {getEnableCommandsTerminalInstruction(target())}
                            </p>
                            <div class="flex shrink-0 items-center gap-1.5">
                              <Button
                                type="button"
                                variant="outline"
                                size="xs"
                                class="gap-1"
                                onClick={() => setShowCommand((value) => !value)}
                              >
                                <CodeIcon class="h-3.5 w-3.5" aria-hidden="true" />
                                <span>{showCommand() ? 'Hide command' : 'View command'}</span>
                              </Button>
                              <Button
                                type="button"
                                variant="primary"
                                size="xs"
                                class="gap-1"
                                onClick={() => void handleCopyCommand()}
                              >
                                <Show
                                  when={commandCopied()}
                                  fallback={<CopyIcon class="h-3.5 w-3.5" aria-hidden="true" />}
                                >
                                  <CheckIcon class="h-3.5 w-3.5" aria-hidden="true" />
                                </Show>
                                <span>{commandCopied() ? 'Copied' : 'Copy command'}</span>
                              </Button>
                            </div>
                          </div>
                          <Show when={showCommand()}>
                            <pre
                              tabIndex={0}
                              data-testid="enable-remote-commands-script"
                              class="max-h-36 overflow-auto rounded-md border border-border bg-page p-3 font-mono text-xs text-base-content focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500"
                            >
                              <code>{commandScript()}</code>
                            </pre>
                          </Show>
                        </div>

                        <Show when={generatedToken() && platform() !== 'windows'}>
                          <div class="space-y-2 rounded-lg border border-border bg-surface-alt/60 p-3">
                            <div class="flex flex-wrap items-center justify-between gap-2">
                              <p class="text-xs font-medium text-base-content">
                                2. Paste this token at the silent prompt
                              </p>
                              <div class="flex shrink-0 items-center gap-1.5">
                                <Button
                                  type="button"
                                  variant="outline"
                                  size="xs"
                                  class="gap-1"
                                  onClick={() => setShowToken((value) => !value)}
                                >
                                  <Show
                                    when={showToken()}
                                    fallback={<EyeIcon class="h-3.5 w-3.5" aria-hidden="true" />}
                                  >
                                    <EyeOffIcon class="h-3.5 w-3.5" aria-hidden="true" />
                                  </Show>
                                  <span>{showToken() ? 'Hide token' : 'Show token'}</span>
                                </Button>
                                <Button
                                  type="button"
                                  variant="secondary"
                                  size="xs"
                                  class="gap-1"
                                  onClick={() => void handleCopyToken()}
                                >
                                  <Show
                                    when={tokenCopied()}
                                    fallback={<CopyIcon class="h-3.5 w-3.5" aria-hidden="true" />}
                                  >
                                    <CheckIcon class="h-3.5 w-3.5" aria-hidden="true" />
                                  </Show>
                                  <span>{tokenCopied() ? 'Copied' : 'Copy token'}</span>
                                </Button>
                              </div>
                            </div>
                            <Show when={showToken()}>
                              <code
                                class="block break-all rounded-sm border border-border bg-page px-2.5 py-1.5 font-mono text-xs text-base-content select-all"
                                data-testid="enable-remote-commands-token"
                              >
                                {generatedToken()}
                              </code>
                            </Show>
                          </div>
                        </Show>

                        <Show when={isHttps()}>
                          <details
                            class="rounded-lg border border-border bg-surface-alt/40 px-3 py-2 text-xs"
                            data-testid="enable-remote-commands-tls-options"
                          >
                            <summary class="cursor-pointer font-medium text-muted hover:text-base-content">
                              Self-signed HTTPS certificate?
                            </summary>
                            <div class="mt-2.5 space-y-2.5 border-t border-border-subtle pt-2.5">
                              <label class="flex cursor-pointer items-start gap-2 text-xs text-base-content">
                                <input
                                  type="checkbox"
                                  checked={insecureMode()}
                                  onChange={(event) => setInsecureMode(event.currentTarget.checked)}
                                  class="mt-0.5 rounded-sm text-blue-600 focus:ring-blue-500"
                                />
                                <span>Skip TLS certificate verification (--insecure)</span>
                              </label>
                              <div class="space-y-1">
                                <label
                                  for={caPathInputId}
                                  class="block text-xs font-medium text-muted"
                                >
                                  Custom CA certificate path (optional)
                                </label>
                                <input
                                  id={caPathInputId}
                                  type="text"
                                  value={customCaPath()}
                                  onInput={(event) => setCustomCaPath(event.currentTarget.value)}
                                  placeholder="/etc/ssl/certs/pulse-ca.crt"
                                  class="w-full rounded-md border border-border bg-page px-2.5 py-1.5 font-mono text-xs text-base-content placeholder:text-muted/70 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500"
                                />
                              </div>
                            </div>
                          </details>
                        </Show>

                        <div
                          class="flex items-center gap-2 text-xs text-muted"
                          role="status"
                          data-testid="enable-remote-commands-waiting"
                        >
                          <Loader2Icon
                            class="h-3.5 w-3.5 shrink-0 animate-spin"
                            aria-hidden="true"
                          />
                          <span>
                            Waiting for {target().hostName} to reconnect with remote commands
                            enabled...
                          </span>
                        </div>
                      </div>
                    </Show>
                  </Show>
                </Show>
              </Show>
            </div>

            <div class="flex shrink-0 flex-wrap items-center justify-between gap-2 border-t border-border px-5 py-3">
              <div>
                <Show when={!isEnabled() && !isLeastPrivilege()}>
                  <a
                    href={installerHref()}
                    onClick={props.onClose}
                    class="inline-flex items-center gap-1 text-xs font-medium text-muted underline-offset-2 hover:text-base-content hover:underline"
                  >
                    <span>Open full installer</span>
                    <ExternalLinkIcon class="h-3 w-3" aria-hidden="true" />
                  </a>
                </Show>
              </div>
              <div class="flex items-center gap-2">
                <Show
                  when={isEnabled()}
                  fallback={
                    <>
                      <Button type="button" variant="outline" size="sm" onClick={props.onClose}>
                        Close
                      </Button>
                      <Show when={isLeastPrivilege()}>
                        <a
                          href={agentDoctorHref()}
                          class={getButtonClass({ variant: 'primary', size: 'sm' })}
                          onClick={props.onClose}
                        >
                          Open Agent Doctor
                        </a>
                      </Show>
                    </>
                  }
                >
                  <Button type="button" variant="outline" size="sm" onClick={props.onClose}>
                    Done
                  </Button>
                  <Show when={props.onContinue}>
                    {(onContinue) => (
                      <Button
                        type="button"
                        variant="primary"
                        size="sm"
                        onClick={() => {
                          onContinue()();
                          props.onClose();
                        }}
                      >
                        {getEnableCommandsContinueLabel(target())}
                      </Button>
                    )}
                  </Show>
                </Show>
              </div>
            </div>
          </div>
        )}
      </Show>
    </Dialog>
  );
};

export default EnableRemoteCommandsDialog;
