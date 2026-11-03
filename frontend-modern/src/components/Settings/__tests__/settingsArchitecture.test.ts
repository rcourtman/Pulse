import { describe, expect, it } from 'vitest';
import { sanitizeDiagnosticsData } from '../diagnosticsModel';
import type { DiagnosticsData } from '../diagnosticsModel';
import settingsSource from '../Settings.tsx?raw';
import { EN_MESSAGES } from '@/i18n/messages';
import aiSettingsSource from '../AISettings.tsx?raw';
import aiModelSelectionSectionSource from '../AIModelSelectionSection.tsx?raw';
import aiProviderConfigurationSectionSource from '../AIProviderConfigurationSection.tsx?raw';
import aiSettingsModelSource from '../aiSettingsModel.ts?raw';
import generalSettingsPanelSource from '../GeneralSettingsPanel.tsx?raw';
import settingsAccessSource from '../useSettingsAccess.ts?raw';
import aiSettingsStateSource from '../useAISettingsState.ts?raw';
import settingsPanelRegistryContextSource from '../settingsPanelRegistryContext.tsx?raw';
import auditLogPanelSource from '../AuditLogPanel.tsx?raw';
import securityAuthPanelSource from '../SecurityAuthPanel.tsx?raw';
import securityOverviewPanelSource from '../SecurityOverviewPanel.tsx?raw';
import updatesSettingsPanelSource from '../UpdatesSettingsPanel.tsx?raw';
import infrastructureWorkspaceSource from '../InfrastructureWorkspace.tsx?raw';
import infrastructureInstallerSectionSource from '../InfrastructureInstallerSection.tsx?raw';
import nodeCredentialSlotSource from '../ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx?raw';
import nodeModalSetupGuideSectionSource from '../NodeModalSetupGuideSection.tsx?raw';
import nodeModalStateSource from '../useNodeModalState.ts?raw';
import trueNASCredentialSlotSource from '../ConnectionEditor/CredentialSlots/TrueNASCredentialSlot.tsx?raw';
import vmwareCredentialSlotSource from '../ConnectionEditor/CredentialSlots/VMwareCredentialSlot.tsx?raw';
import organizationAccessMembersSectionSource from '../OrganizationAccessMembersSection.tsx?raw';
import organizationIncomingSharesSectionSource from '../OrganizationIncomingSharesSection.tsx?raw';
import organizationOutgoingSharesSectionSource from '../OrganizationOutgoingSharesSection.tsx?raw';
import organizationOverviewMembersSectionSource from '../OrganizationOverviewMembersSection.tsx?raw';
import rolesPanelSource from '../RolesPanel.tsx?raw';
import userAssignmentsPanelSource from '../UserAssignmentsPanel.tsx?raw';
import ssoProvidersPanelSource from '../SSOProvidersPanel.tsx?raw';
import ssoProvidersStateSource from '../useSSOProvidersState.ts?raw';
import useInfrastructureInstallStateSource from '../useInfrastructureInstallState.tsx?raw';
import useInfrastructureConfiguredNodesStateSource from '../useInfrastructureConfiguredNodesState.ts?raw';
import useInfrastructureSettingsStateSource from '../useInfrastructureSettingsState.ts?raw';
import ssoProviderPresentationSource from '../../../utils/ssoProviderPresentation.ts?raw';

const settingsRuntimeSources = import.meta.glob(['../*.tsx', '../ConnectionEditor/**/*.tsx'], {
  query: '?raw',
  eager: true,
  import: 'default',
}) as Record<string, string>;

describe('settings architecture guardrails', () => {
  it('keeps the Proxmox installer TLS override explicit and request-bound', () => {
    expect(nodeModalSetupGuideSectionSource).toContain(
      'Skip TLS certificate verification when downloading the installer and',
    );
    expect(nodeModalSetupGuideSectionSource).toContain('connecting the agent to Pulse');
    expect(nodeModalSetupGuideSectionSource).toContain('state.agentInstallInsecure()');
    expect(nodeModalSetupGuideSectionSource).toContain('state.setAgentInstallInsecure');
    expect(nodeModalStateSource).toContain('insecure: agentInstallInsecure()');
  });

  it('gates the Infrastructure nav item on the served infrastructureRead capability', () => {
    // The nav gate itself is exercised in infrastructureNavCapabilityGate.test.ts.
    // DEFAULT_SETTINGS_TAB points at the tab that gate made blockable, so the
    // fallback must resolve a reachable tab rather than the hardcoded default,
    // or a non-admin lands back on the blocked route.
    expect(settingsAccessSource).toContain('SETTINGS_FALLBACK_TAB_ORDER.find(');
    expect(settingsAccessSource).toContain('setActiveTab(fallbackTab)');
  });

  it('keeps capability-gated panels fail-closed before their routes resolve', () => {
    expect(settingsSource).toContain(
      'canMountSettingsPanel(currentTab, securityStatus()?.settingsCapabilities)',
    );
    expect(settingsAccessSource).toMatch(
      /securityStatus\(\) !== null \|\| !securityStatusLoading\(\) \|\| securityStatusError\(\)/,
    );
    expect(settingsAccessSource).toContain("DEFAULT_SETTINGS_TAB,\n  'system-general'");
  });

  it('keeps viewer sessions on the runtime-display settings projection when infrastructure read is denied', () => {
    // system-general stays ungated (previous test), so a session without
    // infrastructureRead still renders the General panel. The system-settings
    // state must therefore still initialize on mount — it degrades to the
    // /api/runtime/display projection internally when the admin payload 403s.
    // Returning before that init is what left issue #1601's viewers reading
    // "Realtime (10s)" while the server polled at the configured cadence.
    const gatedBranch = useInfrastructureSettingsStateSource.match(
      /if \(!canReadInfrastructure\(\)\) \{[\s\S]*?\n {6}\}/,
    );
    expect(gatedBranch?.[0]).toContain('await initializeSystemSettingsState();');
  });

  it('keeps retained settings inventory scoped to configured agent nodes', () => {
    expect(useInfrastructureSettingsStateSource).toContain('canReadInfrastructure,');
    expect(useInfrastructureConfiguredNodesStateSource).toContain("query: 'type=agent'");
    expect(useInfrastructureConfiguredNodesStateSource).toContain(
      "cacheKey: 'settings-configured-nodes'",
    );
    expect(useInfrastructureConfiguredNodesStateSource).toContain(
      "initialHydration: 'prefer-ws-then-rest'",
    );
    expect(useInfrastructureConfiguredNodesStateSource).toContain('enabled: canReadInfrastructure');
    expect(useInfrastructureConfiguredNodesStateSource).toContain(
      'configuredNodeResources.aggregations()?.byType[type]',
    );
    expect(useInfrastructureConfiguredNodesStateSource).not.toContain('useResources()');
  });

  it('renders security restart guidance from the shared presenter', () => {
    // Restart guidance comes from the shared presenter for every deployment
    // type, not from a hand-written block per type.
    expect(securityAuthPanelSource).toContain('{restartInstruction().label}');
    expect(securityAuthPanelSource).not.toContain('deploymentType ===');
    expect(securityAuthPanelSource).not.toContain('ProxmoxVE console');
  });

  it('keeps dense settings tables on deliberate phone information priorities', () => {
    const expectHiddenColumn = (source: string, key: string) => {
      expect(source).toMatch(
        new RegExp(`key: '${key}',[\\s\\S]*?label: '[^']+',[\\s\\S]*?hiddenOnMobile: true`),
      );
    };

    expectHiddenColumn(organizationAccessMembersSectionSource, 'addedAt');
    expectHiddenColumn(organizationOverviewMembersSectionSource, 'addedAt');
    expectHiddenColumn(organizationIncomingSharesSectionSource, 'accessRole');
    expectHiddenColumn(organizationIncomingSharesSectionSource, 'createdAt');
    expectHiddenColumn(organizationOutgoingSharesSectionSource, 'accessRole');
    expectHiddenColumn(organizationOutgoingSharesSectionSource, 'createdAt');
    expectHiddenColumn(rolesPanelSource, 'permissions');
    expectHiddenColumn(auditLogPanelSource, 'success');
    expectHiddenColumn(auditLogPanelSource, 'timestamp');
    expectHiddenColumn(auditLogPanelSource, 'user');

    expect(auditLogPanelSource).toContain('text-[11px] text-muted sm:hidden');
    expect(auditLogPanelSource).toContain("event.user || 'Unknown user'");
    expect(userAssignmentsPanelSource).toContain(
      '<span class="hidden sm:inline">Manage Access</span>',
    );
    expect(userAssignmentsPanelSource).toContain('<span class="hidden sm:inline">Remove</span>');
    // Scheduled reports use a list. ReportingPanel.test.tsx and the Reports
    // browser matrix verify visible recovery actions instead of table markup.
  });

  it('normalizes sparse security status before deriving privileged posture', () => {
    expect(securityAuthPanelSource).toContain('apiTokenConfigured?: boolean;');
    expect(securityOverviewPanelSource).toContain(
      'const postureStatus = createMemo<SecurityPostureStatus | null>',
    );
    expect(securityOverviewPanelSource).toContain(
      'apiTokenConfigured: status.apiTokenConfigured === true',
    );
    expect(securityOverviewPanelSource).toContain(
      'exportProtected: status.exportProtected === true',
    );
    expect(securityOverviewPanelSource).toContain(
      'hasAuditLogging: status.hasAuditLogging === true',
    );
    expect(securityOverviewPanelSource).toContain(
      'postureStatus() ? getSecurityHardeningActions(postureStatus()!) : []',
    );
  });

  it('keeps telemetry disclosure aligned with the security privacy contract', () => {
    expect(EN_MESSAGES['settings.general.telemetry.description']).toContain(
      'coarse deployment and lifecycle buckets',
    );
    expect(EN_MESSAGES['settings.general.telemetry.description']).toContain(
      'aggregate resource and outcome counts, coarse feature flags, content-free Patrol, Assistant, and capability-API usage counters, and four session-deduplicated workload-history adoption counts',
    );
    expect(EN_MESSAGES['settings.general.telemetry.description']).not.toContain(
      'Pulse Intelligence loop adoption',
    );
    expect(EN_MESSAGES['settings.general.telemetry.description']).toContain(
      'identifiers, URLs, paths, locale, raw browser events, an event-level clickstream, prompts, chat messages, command text, action output, token values, names, email addresses, or IP addresses',
    );
    expect(generalSettingsPanelSource).not.toContain('license_tier');
    expect(generalSettingsPanelSource).not.toContain('api_tokens');
  });

  it('reveals Patrol entry points after AI settings saves without a page reload', () => {
    // Every successful AI settings save path must re-derive the session
    // assistantEnabled capability so the Patrol tab and alert Investigate
    // buttons appear (or disappear) mid-session.
    const refreshCalls = aiSettingsStateSource.match(
      /aiAvailabilityStore\.refreshEnabledFromServer\(\)/g,
    );
    expect(refreshCalls?.length ?? 0).toBeGreaterThanOrEqual(3);
    expect(aiSettingsStateSource).not.toContain('aiChatStore');
    expect(aiSettingsStateSource).not.toContain(
      'Pulse Intelligence enabled. You can customize settings below.',
    );
  });

  it('keeps the language selector registry-driven and stacked on phones', () => {
    expect(generalSettingsPanelSource).toContain('SUPPORTED_LOCALES.map((locale) => ({');
    expect(generalSettingsPanelSource).toContain(
      'class="flex flex-col items-start gap-2 p-2 sm:flex-row sm:items-center sm:justify-between sm:gap-4 sm:p-6"',
    );
    expect(generalSettingsPanelSource).toContain(
      'class="max-w-full shrink-0 [&_button]:px-1.5 sm:w-auto sm:[&_button]:px-3"',
    );
  });

  it('keeps the Proxmox agent install request command-free and its setup token off the clipboard', () => {
    // An agent installed from the node editor must not get remote command
    // execution, and the clipboard carries only the credential-free command;
    // the one-time setup token is shown separately.
    expect(nodeModalStateSource).toContain('copyToClipboard(data.command)');
    expect(nodeModalStateSource).toContain('showSetupToken(data)');
    expect(nodeModalStateSource).toContain('enableCommands: false');
  });

  it('keeps Ollama credentials in the provider editor with write-only saved state', () => {
    expect(aiProviderConfigurationSectionSource).toContain('props.settings()?.ollama_password_set');
    expect(aiProviderConfigurationSectionSource).toContain('Clear saved Ollama password');
    expect(aiProviderConfigurationSectionSource).not.toMatch(
      /props\.settings\(\)\?\.ollama_password\b/,
    );
    expect(aiSettingsModelSource).toContain("inputField: 'ollamaPassword'");
    expect(aiSettingsStateSource).toContain("ollamaPassword: ''");
    expect(aiSettingsStateSource).not.toMatch(/data\.ollama_password\b/);
    expect(aiSettingsStateSource).toContain('payload.ollama_password = form.ollamaPassword;');
    expect(aiSettingsStateSource).toContain('payload.clear_ollama_password = true;');
    expect(aiSettingsStateSource).not.toContain('form.ollamaPassword.trim()');
  });

  it("passes the form's pending patrolModel to the advisor so the model check tests the unsaved selection", () => {
    // Without this, clicking Check Patrol model after changing the model
    // dropdown silently tested the previously-saved model and the
    // operator would believe their pending selection was verified.
    expect(aiSettingsStateSource).toContain('form.patrolModel');
    expect(aiSettingsStateSource).toContain('pendingModel');
    expect(aiSettingsStateSource).toContain('runPatrolModelReadiness(');
    expect(aiSettingsStateSource).toContain('pendingModel ? { model: pendingModel } : {}');
  });

  it("flags the inline preflight panel as stale when the cached result is for a different model than the form's current selection", () => {
    // Cache may hold a green result for the previously-saved model
    // while the operator has changed the dropdown. Show a warning-tone
    // panel with copy that names both models so the green badge
    // doesn't silently mislead.
    expect(aiModelSelectionSectionSource).toContain('isStaleAgainstFormSelection');
    expect(aiModelSelectionSectionSource).toContain('pendingFormModel');
    expect(aiModelSelectionSectionSource).toContain('cachedResultModel');
    expect(aiModelSelectionSectionSource).toContain('Evaluation result is for');
    expect(aiModelSelectionSectionSource).toContain(
      'Click Check Patrol model to test the pending selection',
    );
  });

  it('keeps the SSO editor header outside its independently scrolling fields', () => {
    const editor = ssoProvidersPanelSource.slice(
      ssoProvidersPanelSource.indexOf('{/* Add/Edit Modal */}'),
      ssoProvidersPanelSource.indexOf('{/* Delete confirmation modal */}'),
    );
    expect(editor).toContain('min-h-0 flex-col');
    expect(editor).toContain('shrink-0 border-b border-border bg-surface');
    expect(editor).toContain('min-h-0 flex-1 space-y-4 overflow-y-auto');
    expect(editor).not.toContain('sticky');
  });

  it('keeps metadata preview recovery on private dialog-owned presentation', () => {
    const start = ssoProvidersStateSource.indexOf('const fetchMetadataPreview =');
    const previewLifecycle = ssoProvidersStateSource.slice(
      start,
      ssoProvidersStateSource.indexOf('\n  return {', start),
    );
    expect(previewLifecycle).toContain('apiErrorFromResponse(response)');
    expect(previewLifecycle).toContain('getSSOMetadataFetchErrorMessage(error)');
    expect(previewLifecycle).toContain('getSSOMetadataFetchErrorMessage()');
    expect(previewLifecycle).toContain('setEditorFeedback');
    expect(ssoProvidersPanelSource).toContain('returnFocus={() => metadataPreviewButton}');
    expect(previewLifecycle).toContain('...requestOptions(controller)');
    const requestPolicy = ssoProvidersStateSource.slice(
      ssoProvidersStateSource.indexOf('const requestOptions ='),
      ssoProvidersStateSource.indexOf('const clearPreviewAndTest ='),
    );
    expect(requestPolicy).toContain('retry: false');
    expect(requestPolicy).toContain('expectedOrgID: selectedOrg');
    expect(requestPolicy).toContain('signal: controller.signal');
    expect(settingsPanelRegistryContextSource).toMatch(
      /<SSOProvidersPanel\s+currentUser=\{currentUser\(\)\}/,
    );
    expect(previewLifecycle).not.toContain('response.text()');
    expect(previewLifecycle).not.toContain('String(error)');
    expect(previewLifecycle).not.toContain('notificationStore');
    expect(previewLifecycle).not.toMatch(/logger\.error\([^;]*,\s*error\)/);
    const presentation = ssoProviderPresentationSource.slice(
      ssoProviderPresentationSource.indexOf('export function getSSOMetadataFetchErrorMessage'),
      ssoProviderPresentationSource.indexOf('export function getSSOProviderSummary'),
    );
    expect(presentation).not.toContain('String(error)');
    expect(presentation).not.toContain('error.message');
  });

  it('keeps card endpoint labels outside the wrapping shared copy control', () => {
    const endpoint = ssoProvidersPanelSource.slice(
      ssoProvidersPanelSource.indexOf('const SSOEndpoint:'),
      ssoProvidersPanelSource.indexOf('interface SSOProvidersPanelProps'),
    );
    // The old inline flex row let a non-shrinking URL squeeze its label and
    // disappear beyond the card. Keep this shared card projection explicit;
    // rendered/browser checks cover copying and actual responsive geometry.
    expect(endpoint).toContain('<dt class="text-muted">{props.caption}</dt>');
    expect(endpoint).toContain('value={props.value}');
    expect(endpoint).toContain('onCopyValue={props.onCopyValue}');
    expect(endpoint).toContain('w-full max-w-full');
    expect(endpoint).toContain('[overflow-wrap:anywhere]');
    expect(endpoint).toContain('[&>svg]:shrink-0');
    expect(endpoint).not.toContain('truncate');
    expect(ssoProvidersPanelSource.match(/<SSOEndpoint\s/g)).toHaveLength(3);
    expect(ssoProvidersPanelSource).not.toContain('max-w-[18rem]');
  });

  it('keeps settings native select controls on the shared labelled primitive', () => {
    const rawSelectUsers = Object.entries(settingsRuntimeSources)
      .filter(([, source]) => source.includes('<select'))
      .map(([path]) => path)
      .sort();

    expect(rawSelectUsers).toEqual([]);
  });

  it('keeps provider configuration fields out of login credential autofill', () => {
    expect(aiSettingsSource).toContain('<form class="divide-y divide-border" autocomplete="off"');
    expect(aiModelSelectionSectionSource).toContain('autocomplete="off"');
    expect(aiProviderConfigurationSectionSource).toContain(
      "autocomplete={config.inputType === 'password' ? 'new-password' : 'off'}",
    );
    expect(aiProviderConfigurationSectionSource).toContain(
      'aria-label={`${getAIProviderDisplayName(config.provider)} ${extraField.label}`}',
    );
    expect(aiProviderConfigurationSectionSource).toContain(
      "autocomplete={extraField.type === 'password' ? 'new-password' : 'off'}",
    );
  });

  it('keeps Pro-runtime docker updates off the community image commands', () => {
    // A Pro-runtime container updates with digest-pinned commands from the
    // license server broker; pulling the community rcourtman/pulse image
    // would silently downgrade it to the community build. The panel keys the
    // suppression off the compiled runtime identity; UpdateInstallGuide.test.tsx
    // renders the guide's Pro and community command sets.
    expect(updatesSettingsPanelSource).toContain("runtimeCapabilities()?.runtime?.build === 'pro'");
    expect(updatesSettingsPanelSource).toContain('isProRuntime={isProRuntime()}');
  });

  it('keeps setup-handoff token cleanup from masking install failures', () => {
    expect(useInfrastructureInstallStateSource).toContain(
      [
        '} finally {',
        '      if (!disposed) {',
        '        setIsGeneratingToken(false);',
        "        if (source === 'setup_handoff') {",
        '          setSetupHandoffAutoTokenPending(false);',
        '        }',
        '      }',
        '    }',
      ].join('\n'),
    );
    expect(useInfrastructureInstallStateSource).not.toMatch(
      /finally \{[\s\S]*?if \(disposed\) \{[\s\S]*?return;/,
    );
  });

  it('keeps the TLS verification warning on the TrueNAS and vCenter credential slots', () => {
    expect(vmwareCredentialSlotSource).toContain('TlsVerificationWarningBanner');
    expect(vmwareCredentialSlotSource).toContain('subject="this vCenter connection"');
    expect(vmwareCredentialSlotSource).toContain(
      'Install a trusted certificate for vCenter before using this in production.',
    );

    expect(trueNASCredentialSlotSource).toContain('TlsVerificationWarningBanner');
    expect(trueNASCredentialSlotSource).toContain('subject="this TrueNAS connection"');
    expect(trueNASCredentialSlotSource).toContain(
      'Install a trusted certificate or configure the TLS fingerprint before using this in production.',
    );
  });

  it('keeps actual token usage names and IDs inside the same export privacy boundary', () => {
    const payload = {
      apiTokens: {
        enabled: true,
        tokenCount: 1,
        recommendTokenSetup: false,
        tokens: [{ id: 'private-token', name: 'private-purpose' }],
        usage: [{ tokenId: 'private-token', agents: ['private-agent'], agentCount: 1 }],
      },
    } as unknown as DiagnosticsData;
    const sanitized = sanitizeDiagnosticsData(payload);
    expect(JSON.stringify(sanitized)).not.toContain('private-');
    expect(sanitized.apiTokens).toMatchObject({
      tokenCount: 1,
      usage: [{ tokenId: 'token-1', agents: ['docker-host-1'], agentCount: 1 }],
    });
    expect(JSON.stringify(payload)).toContain('private-agent');
  });

  it('keeps every PBS diagnostic failure string inside the export redaction boundary', () => {
    // Behavioural guard, not a source scan: the redaction gap that shipped was
    // a field added to the payload after the sanitizer was written, which no
    // amount of reading the sanitizer's own source would reveal. Assert on the
    // serialized bundle so any future PBS failure string has to be redacted to
    // pass.
    const sanitized = sanitizeDiagnosticsData({
      pbs: [
        {
          id: 'pbs-1',
          name: 'backup',
          host: '192.168.1.20',
          connected: false,
          state: 'unreachable',
          stateReason: 'dial tcp 192.168.1.20:8007 failed',
          error: 'Get https://192.168.1.20:8007: EOF',
          probe: {
            connected: false,
            error: 'dial tcp 192.168.1.20:8007: connect: connection refused',
            troubleshooting: 'Confirm 192.168.1.20 accepts connections on 8007.',
          },
        },
      ],
    } as unknown as DiagnosticsData);

    expect(JSON.stringify(sanitized)).not.toContain('192.168.1.20');
  });
});

describe('node import preview authority boundary', () => {
  it('keeps preview responses informational and setup approval explicit', () => {
    const previewHandler = nodeCredentialSlotSource.slice(
      nodeCredentialSlotSource.indexOf('const handlePreviewImportImpact'),
      nodeCredentialSlotSource.indexOf('const handleSubmit'),
    );
    expect(previewHandler).toContain('MonitoredSystemLedgerAPI.preview(plan.previewRequest)');
    expect(previewHandler).not.toContain('setImportApproved');
    expect(previewHandler).not.toContain('saveNode');
    expect(previewHandler).not.toContain('handleSubmit');
    expect(nodeCredentialSlotSource).toContain(
      'setupHandoffDisabled: () => Boolean(props.importCandidate && !importApproved())',
    );
    expect(nodeCredentialSlotSource).toContain('if (saveBlockedByImportPlan())');
  });
});

describe('ordinary uninstall copy surfaces', () => {
  it('keeps both installer families reachable without an installation token and uses the shared state', () => {
    const removal = infrastructureInstallerSectionSource.slice(
      infrastructureInstallerSectionSource.indexOf('Uninstall agent'),
    );
    expect(removal).toContain('copyToClipboard(state.getUninstallCommand())');
    expect(removal).toContain('copyToClipboard(state.getWindowsUninstallCommand())');
    expect(removal).not.toContain('commandsUnlocked');
    expect(removal).not.toContain('requiresToken');
    expect(removal).not.toContain('handleGenerateToken');
    expect(removal).toContain('saved connection');
    expect(removal).not.toContain("If the agent can't reach this server");
    expect(infrastructureWorkspaceSource).toContain('linux: operations.getUninstallCommand()');
    expect(infrastructureWorkspaceSource).toContain(
      'windows: operations.getWindowsUninstallCommand()',
    );
  });
});

describe('User Access reader projection', () => {
  it('supplies live reader props through the canonical registry instead of an unscoped mount', async () => {
    const { default: registry } = await import('../settingsPanelRegistry.ts?raw');
    const { default: context } = await import('../settingsPanelRegistryContext.tsx?raw');
    expect(registry).toMatch(/'security-roles':\s*\{[^}]*getProps: context.getRolesPanelProps/s);
    expect(registry).toMatch(
      /'security-users':\s*\{[^}]*getProps: context.getUserAssignmentsPanelProps/s,
    );
    expect(context).toMatch(
      /getUserAssignmentsPanelProps: \(\) => \(\{\s*currentUser: currentUser\(\),\s*canManage: settingsCapabilities\(\)\?\.users === true/s,
    );
  });
});
