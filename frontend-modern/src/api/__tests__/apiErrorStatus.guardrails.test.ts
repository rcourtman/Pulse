import { describe, expect, it } from 'vitest';
import aiSource from '@/api/ai.ts?raw';
import aiChatSource from '@/api/aiChat.ts?raw';
import agentProfilesSource from '@/api/agentProfiles.ts?raw';
import alertsSource from '@/api/alerts.ts?raw';
import discoverySource from '@/api/discovery.ts?raw';
import agentMetadataSource from '@/api/agentMetadata.ts?raw';
import hostedSignupSource from '@/api/hostedSignup.ts?raw';
import monitoringSource from '@/api/monitoring.ts?raw';
import notificationsSource from '@/api/notifications.ts?raw';
import nodesSource from '@/api/nodes.ts?raw';
import patrolSource from '@/api/patrol.ts?raw';
import guestMetadataSource from '@/api/guestMetadata.ts?raw';
import metadataClientSource from '@/api/metadataClient.ts?raw';
import responseUtilsSource from '@/api/responseUtils.ts?raw';
import securitySource from '@/api/security.ts?raw';
import streamingSource from '@/api/streaming.ts?raw';

const apiSources = import.meta.glob('../*.ts', {
  query: '?raw',
  eager: true,
  import: 'default',
}) as Record<string, string>;

describe('API error-status guardrails', () => {
  it('keeps canonical API error status helpers in responseUtils', () => {
    expect(responseUtilsSource).toContain('export function apiErrorStatus');
    expect(responseUtilsSource).toContain('export function isAPIErrorStatus');
    expect(responseUtilsSource).toContain('export function apiResponseStatus');
    expect(responseUtilsSource).toContain('export function isAPIResponseStatus');
    expect(responseUtilsSource).toContain('export async function assertAPIResponseOK');
    expect(responseUtilsSource).toContain('export async function assertAPIResponseOKOrAllowedStatus');
    expect(responseUtilsSource).toContain('export async function assertAPIResponseOKOrThrowStatus');
    expect(responseUtilsSource).toContain('export async function parseRequiredAPIResponse');
    expect(responseUtilsSource).toContain('export async function parseOptionalAPIResponse');
    expect(responseUtilsSource).toContain(
      'export async function parseOptionalAPIResponseOrAllowedStatus',
    );
    expect(responseUtilsSource).toContain('export async function parseRequiredAPIResponseOrNull');
    expect(responseUtilsSource).toContain('export async function parseOptionalAPIResponseOrNull');
    expect(responseUtilsSource).toContain('export async function parseRequiredJSON');
    expect(responseUtilsSource).toContain('export async function parseJSONSafe');
    expect(responseUtilsSource).toContain('export function parseJSONTextSafe');
    expect(responseUtilsSource).toContain('export function arrayOrEmpty');
    expect(responseUtilsSource).toContain('export function arrayOrUndefined');
    expect(responseUtilsSource).toContain('export function objectArrayFieldOrEmpty');
    expect(responseUtilsSource).toContain('export function trimmedString');
    expect(responseUtilsSource).toContain('export function optionalTrimmedString');
    expect(responseUtilsSource).toContain('export function strictString');
    expect(responseUtilsSource).toContain('export function strictBoolean');
    expect(responseUtilsSource).toContain('export function finiteNumberOrUndefined');
    expect(responseUtilsSource).toContain('export function coerceTimestampMillis');
    expect(responseUtilsSource).toContain('export function stringArray');
    expect(responseUtilsSource).toContain('export function stringRecordOrUndefined');
    expect(responseUtilsSource).toContain('export function normalizeStructuredAPIError');
    expect(responseUtilsSource).toContain('export function promoteLegacyAlertIdentifier');
    expect(metadataClientSource).toContain('export function buildMetadataAPI');
    expect(metadataClientSource).toContain('export interface ResourceMetadataRecord');
    expect(streamingSource).toContain('export async function consumeJSONEventStream');
    expect(responseUtilsSource).toContain('(error as APIErrorLike).status');
    expect(responseUtilsSource).toContain('response.status');
  });

  it('routes canonical status and JSON parsing through responseUtils', () => {
    expect(aiSource).toContain('isAPIErrorStatus(error, 402)');
    expect(aiSource).toContain('isAPIErrorStatus(error, 404)');
    expect(aiSource).toContain('assertAPIResponseOK(response,');
    expect(aiSource).toContain('consumeJSONEventStream<AIStreamEvent>(response,');
    expect(aiSource).toContain('promoteLegacyAlertIdentifier(');
    expect(aiSource).not.toContain("message.includes('402')");
    expect(aiSource).not.toContain('JSON.parse(');
    expect(aiSource).not.toContain('TextDecoder');
    expect(aiSource).not.toContain('getReader(');
    expect(aiSource).not.toContain('STREAM_TIMEOUT_MS');
    expect(aiSource).not.toContain('private static encodeSegment(');
    expect(aiSource).not.toContain('private static isPaymentRequiredError(');
    expect(aiSource).not.toContain('readAPIErrorMessage(');
    expect(aiSource).not.toContain('} catch {\n      return null;');
    expect(aiSource).not.toContain('normalizeUnifiedFinding(');

    expect(aiChatSource).toContain('assertAPIResponseOK(response,');
    expect(aiChatSource).toContain('consumeJSONEventStream<StreamEvent>(response,');
    expect(aiChatSource).not.toContain('JSON.parse(');
    expect(aiChatSource).not.toContain('TextDecoder');
    expect(aiChatSource).not.toContain('getReader(');
    expect(aiChatSource).not.toContain('STREAM_TIMEOUT_MS');
    expect(aiChatSource).not.toContain('private static encodeSegment(');
    expect(aiChatSource).not.toContain('readAPIErrorMessage(');

    expect(agentProfilesSource).toContain('isAPIErrorStatus(err, 402)');
    expect(agentProfilesSource).toContain('isAPIErrorStatus(err, 404)');
    expect(agentProfilesSource).toContain('assertAPIResponseOKOrAllowedStatus(');
    expect(agentProfilesSource).toContain('assertAPIResponseOKOrThrowStatus(');
    expect(agentProfilesSource).toContain('parseRequiredAPIResponse(');
    expect(agentProfilesSource).toContain('assertAPIResponseOK(response,');
    expect(agentProfilesSource).not.toContain("message.includes('402')");
    expect(agentProfilesSource).not.toContain("message.includes('404')");
    expect(agentProfilesSource).not.toContain('response.status !== 204');
    expect(agentProfilesSource).not.toContain('if (!isAPIResponseStatus(response, 204))');
    expect(agentProfilesSource).not.toContain('response.status === 503');
    expect(agentProfilesSource).not.toContain('return response.json()');
    expect(agentProfilesSource).not.toContain('parseRequiredJSON(response,');
    expect(agentProfilesSource).not.toContain('readAPIErrorMessage(');

    expect(monitoringSource).toContain('parseOptionalAPIResponse(');
    expect(monitoringSource).toContain('parseOptionalAPIResponseOrAllowedStatus(');
    expect(monitoringSource).toContain('parseOptionalAPIResponseOrNull<AgentLookupResponse>(');
    expect(monitoringSource).toContain('assertAPIResponseOK(response,');
    expect(monitoringSource).toContain('assertAPIResponseOKOrAllowedStatus(response, 404,');
    expect(monitoringSource).toContain('assertAPIResponseOKOrThrowStatus(');
    expect(monitoringSource).toContain('coerceTimestampMillis(identity.lastSeen, Date.now())');
    expect(monitoringSource).toContain("'Failed to parse agent lookup response'");
    expect(discoverySource).toContain('assertAPIResponseOK(response,');
    expect(discoverySource).toContain('parseRequiredAPIResponse(');
    expect(discoverySource).toContain('parseRequiredAPIResponseOrNull(');
    expect(discoverySource).toContain('buildTypedDiscoveryPath(');
    expect(discoverySource).toContain('buildAgentDiscoveryCollectionPath(');
    expect(discoverySource).toContain('buildAgentDiscoveryDetailPath(');
    expect(monitoringSource).not.toContain('response.status === 404');
    expect(monitoringSource).not.toContain('JSON.parse(');
    expect(monitoringSource).not.toContain("typeof lastSeen === 'string'");
    expect(monitoringSource).not.toContain('Date.parse(lastSeen)');
    expect(monitoringSource).not.toContain('readAPIErrorMessage(');
    expect(discoverySource).not.toContain('response.status === 404');
    expect(discoverySource).not.toContain('return response.json()');
    expect(discoverySource).not.toContain('const isAgentResourceType =');
    expect(discoverySource).not.toContain('const agentCollectionBasePath =');
    expect(discoverySource).not.toContain('parseRequiredJSON(response,');
    expect(discoverySource).not.toContain('isAPIResponseStatus(response, 404)');
    expect(discoverySource).not.toContain('readAPIErrorMessage(');

    expect(agentMetadataSource).toContain("buildMetadataAPI<AgentMetadata>('/api/agents/metadata')");
    expect(agentMetadataSource).not.toContain('apiFetchJSON(');
    expect(guestMetadataSource).toContain("buildMetadataAPI<GuestMetadata>('/api/guests/metadata')");
    expect(guestMetadataSource).not.toContain('apiFetchJSON(');

    expect(hostedSignupSource).toContain('parseJSONSafe<');
    expect(hostedSignupSource).toContain('normalizeStructuredAPIError(body, response.status)');
    expect(hostedSignupSource).not.toContain('response.json()');
    expect(hostedSignupSource).not.toContain('function normalizeHostedError(');
    expect(hostedSignupSource).not.toContain("typeof obj.code === 'string'");
    expect(hostedSignupSource).not.toContain("typeof obj.message === 'string'");
  });

  it('routes canonical collection normalization through responseUtils', () => {
    expect(aiSource).toContain('arrayOrEmpty<UnifiedFindingRecord>(response.findings)');
    expect(aiSource).toContain("objectArrayFieldOrEmpty<RemediationPlan>(data, 'plans')");
    expect(aiSource).toContain("objectArrayFieldOrEmpty<ApprovalRequest>(response, 'approvals')");
    expect(aiSource).not.toContain('response.approvals || []');
    expect(aiSource).not.toContain('Array.isArray(data?.plans)');

    expect(alertsSource).toContain('arrayOrEmpty<Incident>(incidents)');
    expect(alertsSource).toContain('arrayOrEmpty<{');
    expect(alertsSource).not.toContain('incidents || []');
    expect(alertsSource).not.toContain('response.results || []');
    expect(alertsSource).not.toContain('normalizeAlertResult(');
    expect(alertsSource).not.toContain('normalizeIncident(');
    expect(alertsSource).not.toContain('normalizeIncidents(');

    expect(securitySource).toContain("objectArrayFieldOrEmpty<APITokenRecord>(response, 'tokens')");
    expect(securitySource).not.toContain('response.tokens ?? []');

    expect(notificationsSource).toContain('arrayOrEmpty<Webhook>(data)');
    expect(notificationsSource).not.toContain('Array.isArray(data) ? data : []');

    expect(nodesSource).toContain('arrayOrUndefined<RawClusterEndpoint>(node.clusterEndpoints)');
    expect(nodesSource).not.toContain('Array.isArray(node.clusterEndpoints)');

    expect(patrolSource).toContain('arrayOrEmpty<PatrolRunRecord>(runs)');
    expect(patrolSource).toContain('promoteLegacyAlertIdentifier(');
    expect(patrolSource).not.toContain('runs || []');
    expect(patrolSource).not.toContain('normalizePatrolRunRecord(');

    expect(agentProfilesSource).toContain('arrayOrEmpty<AgentProfile>(response)');
    expect(agentProfilesSource).toContain('arrayOrEmpty<AgentProfileAssignment>(response)');
    expect(agentProfilesSource).toContain('arrayOrEmpty<ConfigKeyDefinitionResponse>(response)');
    expect(agentProfilesSource).toContain(
      "objectArrayFieldOrEmpty<ConfigValidationErrorResponse>(response, 'Errors')",
    );
    expect(agentProfilesSource).not.toContain('response || []');
    expect(agentProfilesSource).not.toContain('response.Errors || []');
  });

  it('routes canonical scalar coercion through responseUtils', () => {
    expect(nodesSource).toContain('trimmedString(endpoint.nodeId)');
    expect(nodesSource).toContain('optionalTrimmedString(endpoint.guestURL)');
    expect(nodesSource).toContain('strictBoolean(endpoint.online)');
    expect(nodesSource).not.toContain('const asString =');
    expect(nodesSource).not.toContain('const asOptionalString =');
    expect(nodesSource).not.toContain('const asBoolean =');

    expect(notificationsSource).toContain('strictString(backendConfig.provider)');
    expect(notificationsSource).toContain('strictBoolean(backendConfig.enabled)');
    expect(notificationsSource).toContain('finiteNumberOrUndefined(backendConfig.port)');
    expect(notificationsSource).toContain('stringArray(backendConfig.to)');
    expect(notificationsSource).not.toContain('private static readString(');
    expect(notificationsSource).not.toContain('private static readBoolean(');
    expect(notificationsSource).not.toContain('private static readNumber(');
    expect(notificationsSource).not.toContain('private static readStringArray(');
  });

  it('bans raw message-based 402/404 heuristics, raw governed response-status checks, raw governed response parsing, module-local collection fallbacks, module-local scalar helper stacks, module-local structured error normalization, module-local timestamp coercion, no-op governed payload wrappers, duplicate legacy alert_identifier promotion, no-op AI helper aliases, raw governed parsed-error throwing, raw governed assert-then-parse pipelines, raw governed 404-null response branches, raw duplicated metadata CRUD clients, raw duplicated SSE stream readers, raw monitoring allowed-status branches, raw agent-profile 204 success branches, raw custom-status error branches, and discovery route alias drift', () => {
    const runtimeEntries = Object.entries(apiSources).filter(
      ([path]) => !path.endsWith('/responseUtils.ts'),
    );
    const rawStatusHeuristicPattern = /message\.includes\((['"])40[24]\1\)/;
    const rawGovernedResponseStatusPattern = /response\.status\s*(?:===|!==)\s*(?:204|404|503)/;
    const governedParsingEntries = runtimeEntries.filter(([path]) =>
      /\/(?:ai|aiChat|agentProfiles|discovery|hostedSignup|monitoring)\.ts$/.test(path),
    );
    const rawResponseJSONPattern = /(?:return\s+)?(?:await\s+)?response\.json\(/;
    const rawManualJSONParsePattern = /JSON\.parse\(/;
    const governedCollectionEntries = runtimeEntries.filter(([path]) =>
      /\/(?:ai|alerts|agentProfiles|nodes|notifications|patrol|security)\.ts$/.test(path),
    );
    const rawArrayFallbackPattern = /(?:\|\||\?\?)\s*\[\]/;
    const rawArrayIsArrayFallbackPattern = /Array\.isArray\(.+\)\s*\?\s*.+:\s*\[\]/;
    const rawOptionalArrayNormalizationPattern = /Array\.isArray\(node\.clusterEndpoints\)/;
    const governedScalarEntries = runtimeEntries.filter(([path]) =>
      /\/(?:nodes|notifications)\.ts$/.test(path),
    );
    const rawScalarHelperPattern =
      /(?:const\s+asString\s*=|const\s+asOptionalString\s*=|const\s+asBoolean\s*=|private\s+static\s+readString\(|private\s+static\s+readBoolean\(|private\s+static\s+readNumber\(|private\s+static\s+readStringArray\()/;
    const governedStructuredErrorEntries = runtimeEntries.filter(([path]) =>
      /\/hostedSignup\.ts$/.test(path),
    );
    const rawStructuredErrorPattern =
      /(?:function\s+normalizeHostedError\(|typeof\s+obj\.code\s*===\s*'string'|typeof\s+obj\.message\s*===\s*'string')/;
    const governedTimestampEntries = runtimeEntries.filter(([path]) =>
      /\/monitoring\.ts$/.test(path),
    );
    const rawTimestampCoercionPattern =
      /(?:typeof\s+lastSeen\s*===\s*'string'|Date\.parse\(lastSeen\)|typeof\s+lastSeen\s*===\s*'number')/;
    const governedWrapperEntries = runtimeEntries.filter(([path]) => /\/alerts\.ts$/.test(path));
    const noOpWrapperPattern =
      /(?:normalizeAlertResult\(|normalizeIncident\(|normalizeIncidents\()/;
    const governedAlertIdentifierEntries = runtimeEntries.filter(([path]) =>
      /\/(?:ai|patrol)\.ts$/.test(path),
    );
    const duplicateAlertIdentifierPattern =
      /(?:normalizeUnifiedFinding\(|normalizePatrolRunRecord\(|alert_identifier:\s*_alertIdentifier|const\s+alertIdentifier\s*=\s*.+alert_identifier)/;
    const governedAiAliasEntries = runtimeEntries.filter(([path]) =>
      /\/(?:ai|aiChat)\.ts$/.test(path),
    );
    const noOpAiAliasPattern =
      /(?:private\s+static\s+encodeSegment\(|private\s+static\s+isPaymentRequiredError\()/;
    const governedDiscoveryRouteEntries = runtimeEntries.filter(([path]) =>
      /\/discovery\.ts$/.test(path),
    );
    const discoveryRouteAliasPattern =
      /(?:const\s+isAgentResourceType\s*=|const\s+agentCollectionBasePath\s*=)/;
    const governedResponseErrorEntries = runtimeEntries.filter(([path]) =>
      /\/(?:ai|aiChat|agentProfiles|discovery|monitoring)\.ts$/.test(path),
    );
    const rawParsedErrorThrowPattern =
      /(?:readAPIErrorMessage\(|throw\s+new\s+Error\(\s*await\s+readAPIErrorMessage\()/;
    const governedAssertParseEntries = runtimeEntries.filter(([path]) =>
      /\/(?:agentProfiles|discovery|monitoring)\.ts$/.test(path),
    );
    const rawAssertThenParsePattern =
      /assertAPIResponseOK\((response|agentListResponse),[\s\S]{0,160}?parse(?:Required|Optional)JSON\(\1,/;
    const governedNullLookupEntries = runtimeEntries.filter(([path]) =>
      /\/(?:discovery|monitoring)\.ts$/.test(path),
    );
    const rawNullLookupPattern =
      /if\s*\(\s*isAPIResponseStatus\(response,\s*404\)\s*\)\s*\{\s*return null;\s*\}/;
    const governedMetadataEntries = runtimeEntries.filter(([path]) =>
      /\/(?:agentMetadata|guestMetadata)\.ts$/.test(path),
    );
    const rawMetadataCrudPattern =
      /(?:apiFetchJSON\(|private static baseUrl = '\/api\/(?:agents|guests)\/metadata')/;
    const governedStreamEntries = runtimeEntries.filter(([path]) =>
      /\/(?:ai|aiChat)\.ts$/.test(path),
    );
    const rawStreamReaderPattern =
      /(?:TextDecoder|getReader\(|STREAM_TIMEOUT_MS|Read timeout|split\('\n\n'\)|startsWith\('data: '\))/;
    const governedMonitoringAllowedStatusEntries = runtimeEntries.filter(([path]) =>
      /\/monitoring\.ts$/.test(path),
    );
    const rawMonitoringAllowedDeletePattern =
      /if\s*\(!response\.ok\)\s*\{\s*if\s*\(isAPIResponseStatus\(response,\s*404\)\)\s*\{\s*return\s+\{\};\s*\}[\s\S]{0,160}?assertAPIResponseOK\([^\n]+\);\s*\}\s*if\s*\(isAPIResponseStatus\(response,\s*204\)\)\s*\{\s*return\s+\{\};\s*\}/;
    const rawMonitoringAllowedMutationPattern =
      /if\s*\(!response\.ok\)\s*\{\s*if\s*\(isAPIResponseStatus\(response,\s*404\)\)\s*\{\s*(?:\/\/[^\n]*\n\s*)?return;\s*\}[\s\S]{0,160}?assertAPIResponseOK\([^\n]+\);\s*\}/;
    const governedAgentProfileAllowedStatusEntries = runtimeEntries.filter(([path]) =>
      /\/agentProfiles\.ts$/.test(path),
    );
    const rawAgentProfileAllowedStatusPattern =
      /if\s*\(!isAPIResponseStatus\(response,\s*204\)\)\s*\{\s*await\s+assertAPIResponseOK\(response,\s*`Failed to (?:delete profile|unassign profile): \$\{response\.status\}`\);\s*\}/;
    const governedCustomStatusErrorEntries = runtimeEntries.filter(([path]) =>
      /\/(?:agentProfiles|monitoring)\.ts$/.test(path),
    );
    const rawCustomStatusErrorPattern =
      /if\s*\(!response\.ok\)\s*\{\s*if\s*\(isAPIResponseStatus\(response,\s*(?:404|503)\)\)\s*\{\s*throw\s+new\s+Error\(/;

    const heuristicOffenders = runtimeEntries
      .filter(([, source]) => rawStatusHeuristicPattern.test(source))
      .map(([path]) => path)
      .sort();

    const responseStatusOffenders = runtimeEntries
      .filter(([, source]) => rawGovernedResponseStatusPattern.test(source))
      .map(([path]) => path)
      .sort();

    const responseJSONOffenders = governedParsingEntries
      .filter(([, source]) => rawResponseJSONPattern.test(source))
      .map(([path]) => path)
      .sort();

    const manualJSONParseOffenders = governedParsingEntries
      .filter(([, source]) => rawManualJSONParsePattern.test(source))
      .map(([path]) => path)
      .sort();

    const arrayFallbackOffenders = governedCollectionEntries
      .filter(
        ([, source]) =>
          rawArrayFallbackPattern.test(source) ||
          rawArrayIsArrayFallbackPattern.test(source) ||
          rawOptionalArrayNormalizationPattern.test(source),
      )
      .map(([path]) => path)
      .sort();

    const scalarHelperOffenders = governedScalarEntries
      .filter(([, source]) => rawScalarHelperPattern.test(source))
      .map(([path]) => path)
      .sort();

    const structuredErrorOffenders = governedStructuredErrorEntries
      .filter(([, source]) => rawStructuredErrorPattern.test(source))
      .map(([path]) => path)
      .sort();

    const timestampCoercionOffenders = governedTimestampEntries
      .filter(([, source]) => rawTimestampCoercionPattern.test(source))
      .map(([path]) => path)
      .sort();

    const noOpWrapperOffenders = governedWrapperEntries
      .filter(([, source]) => noOpWrapperPattern.test(source))
      .map(([path]) => path)
      .sort();

    const duplicateAlertIdentifierOffenders = governedAlertIdentifierEntries
      .filter(([, source]) => duplicateAlertIdentifierPattern.test(source))
      .map(([path]) => path)
      .sort();

    const noOpAiAliasOffenders = governedAiAliasEntries
      .filter(([, source]) => noOpAiAliasPattern.test(source))
      .map(([path]) => path)
      .sort();

    const discoveryRouteAliasOffenders = governedDiscoveryRouteEntries
      .filter(([, source]) => discoveryRouteAliasPattern.test(source))
      .map(([path]) => path)
      .sort();

    const rawParsedErrorThrowOffenders = governedResponseErrorEntries
      .filter(([, source]) => rawParsedErrorThrowPattern.test(source))
      .map(([path]) => path)
      .sort();

    const rawAssertThenParseOffenders = governedAssertParseEntries
      .filter(([, source]) => rawAssertThenParsePattern.test(source))
      .map(([path]) => path)
      .sort();

    const rawNullLookupOffenders = governedNullLookupEntries
      .filter(([, source]) => rawNullLookupPattern.test(source))
      .map(([path]) => path)
      .sort();

    const rawMetadataCrudOffenders = governedMetadataEntries
      .filter(([, source]) => rawMetadataCrudPattern.test(source))
      .map(([path]) => path)
      .sort();

    const rawStreamReaderOffenders = governedStreamEntries
      .filter(([, source]) => rawStreamReaderPattern.test(source))
      .map(([path]) => path)
      .sort();

    const rawMonitoringAllowedStatusOffenders = governedMonitoringAllowedStatusEntries
      .filter(
        ([, source]) =>
          rawMonitoringAllowedDeletePattern.test(source) ||
          rawMonitoringAllowedMutationPattern.test(source),
      )
      .map(([path]) => path)
      .sort();

    const rawAgentProfileAllowedStatusOffenders = governedAgentProfileAllowedStatusEntries
      .filter(([, source]) => rawAgentProfileAllowedStatusPattern.test(source))
      .map(([path]) => path)
      .sort();

    const rawCustomStatusErrorOffenders = governedCustomStatusErrorEntries
      .filter(([, source]) => rawCustomStatusErrorPattern.test(source))
      .map(([path]) => path)
      .sort();

    expect(heuristicOffenders).toEqual([]);
    expect(responseStatusOffenders).toEqual([]);
    expect(responseJSONOffenders).toEqual([]);
    expect(manualJSONParseOffenders).toEqual([]);
    expect(arrayFallbackOffenders).toEqual([]);
    expect(scalarHelperOffenders).toEqual([]);
    expect(structuredErrorOffenders).toEqual([]);
    expect(timestampCoercionOffenders).toEqual([]);
    expect(noOpWrapperOffenders).toEqual([]);
    expect(duplicateAlertIdentifierOffenders).toEqual([]);
    expect(noOpAiAliasOffenders).toEqual([]);
    expect(rawParsedErrorThrowOffenders).toEqual([]);
    expect(rawAssertThenParseOffenders).toEqual([]);
    expect(rawNullLookupOffenders).toEqual([]);
    expect(rawMetadataCrudOffenders).toEqual([]);
    expect(rawStreamReaderOffenders).toEqual([]);
    expect(rawMonitoringAllowedStatusOffenders).toEqual([]);
    expect(rawAgentProfileAllowedStatusOffenders).toEqual([]);
    expect(rawCustomStatusErrorOffenders).toEqual([]);
    expect(discoveryRouteAliasOffenders).toEqual([]);
  });
});
