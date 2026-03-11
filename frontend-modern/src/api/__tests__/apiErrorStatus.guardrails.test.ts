import { describe, expect, it } from 'vitest';
import aiSource from '@/api/ai.ts?raw';
import aiChatSource from '@/api/aiChat.ts?raw';
import agentProfilesSource from '@/api/agentProfiles.ts?raw';
import alertsSource from '@/api/alerts.ts?raw';
import discoverySource from '@/api/discovery.ts?raw';
import hostedSignupSource from '@/api/hostedSignup.ts?raw';
import monitoringSource from '@/api/monitoring.ts?raw';
import notificationsSource from '@/api/notifications.ts?raw';
import nodesSource from '@/api/nodes.ts?raw';
import patrolSource from '@/api/patrol.ts?raw';
import responseUtilsSource from '@/api/responseUtils.ts?raw';
import securitySource from '@/api/security.ts?raw';

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
    expect(responseUtilsSource).toContain('(error as APIErrorLike).status');
    expect(responseUtilsSource).toContain('response.status');
  });

  it('routes canonical status and JSON parsing through responseUtils', () => {
    expect(aiSource).toContain('isAPIErrorStatus(error, 402)');
    expect(aiSource).toContain('isAPIErrorStatus(error, 404)');
    expect(aiSource).toContain('parseJSONTextSafe<AIStreamEvent>(');
    expect(aiSource).toContain('promoteLegacyAlertIdentifier(');
    expect(aiSource).not.toContain("message.includes('402')");
    expect(aiSource).not.toContain('JSON.parse(');
    expect(aiSource).not.toContain('private static encodeSegment(');
    expect(aiSource).not.toContain('private static isPaymentRequiredError(');
    expect(aiSource).not.toContain('} catch {\n      return null;');
    expect(aiSource).not.toContain('normalizeUnifiedFinding(');

    expect(aiChatSource).toContain('parseJSONTextSafe<StreamEvent>(');
    expect(aiChatSource).not.toContain('JSON.parse(');
    expect(aiChatSource).not.toContain('private static encodeSegment(');

    expect(agentProfilesSource).toContain('isAPIErrorStatus(err, 402)');
    expect(agentProfilesSource).toContain('isAPIErrorStatus(err, 404)');
    expect(agentProfilesSource).toContain('isAPIResponseStatus(response, 204)');
    expect(agentProfilesSource).toContain('isAPIResponseStatus(response, 503)');
    expect(agentProfilesSource).toContain('parseRequiredJSON(response,');
    expect(agentProfilesSource).not.toContain("message.includes('402')");
    expect(agentProfilesSource).not.toContain("message.includes('404')");
    expect(agentProfilesSource).not.toContain('response.status !== 204');
    expect(agentProfilesSource).not.toContain('response.status === 503');
    expect(agentProfilesSource).not.toContain('return response.json()');

    expect(monitoringSource).toContain('isAPIResponseStatus(response, 404)');
    expect(monitoringSource).toContain('parseOptionalJSON<AgentLookupResponse | null>(');
    expect(monitoringSource).toContain('coerceTimestampMillis(identity.lastSeen, Date.now())');
    expect(monitoringSource).toContain("'Failed to parse agent lookup response'");
    expect(discoverySource).toContain('isAPIResponseStatus(response, 404)');
    expect(discoverySource).toContain('parseRequiredJSON(response,');
    expect(monitoringSource).not.toContain('response.status === 404');
    expect(monitoringSource).not.toContain('JSON.parse(');
    expect(monitoringSource).not.toContain("typeof lastSeen === 'string'");
    expect(monitoringSource).not.toContain('Date.parse(lastSeen)');
    expect(discoverySource).not.toContain('response.status === 404');
    expect(discoverySource).not.toContain('return response.json()');

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

  it('bans raw message-based 402/404 heuristics, raw governed response-status checks, raw governed response parsing, module-local collection fallbacks, module-local scalar helper stacks, module-local structured error normalization, module-local timestamp coercion, no-op governed payload wrappers, duplicate legacy alert_identifier promotion, and no-op AI helper aliases', () => {
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
  });
});
