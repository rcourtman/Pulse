import { describe, expect, it } from 'vitest';
import aiSource from '@/api/ai.ts?raw';
import aiChatSource from '@/api/aiChat.ts?raw';
import agentProfilesSource from '@/api/agentProfiles.ts?raw';
import alertsSource from '@/api/alerts.ts?raw';
import discoverySource from '@/api/discovery.ts?raw';
import hostedSignupSource from '@/api/hostedSignup.ts?raw';
import monitoringSource from '@/api/monitoring.ts?raw';
import notificationsSource from '@/api/notifications.ts?raw';
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
    expect(responseUtilsSource).toContain('export function objectArrayFieldOrEmpty');
    expect(responseUtilsSource).toContain('(error as APIErrorLike).status');
    expect(responseUtilsSource).toContain('response.status');
  });

  it('routes canonical status and JSON parsing through responseUtils', () => {
    expect(aiSource).toContain('isAPIErrorStatus(error, 402)');
    expect(aiSource).toContain('parseJSONTextSafe<AIStreamEvent>(');
    expect(aiSource).not.toContain("message.includes('402')");
    expect(aiSource).not.toContain('JSON.parse(');

    expect(aiChatSource).toContain('parseJSONTextSafe<StreamEvent>(');
    expect(aiChatSource).not.toContain('JSON.parse(');

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
    expect(monitoringSource).toContain("'Failed to parse agent lookup response'");
    expect(discoverySource).toContain('isAPIResponseStatus(response, 404)');
    expect(discoverySource).toContain('parseRequiredJSON(response,');
    expect(monitoringSource).not.toContain('response.status === 404');
    expect(monitoringSource).not.toContain('JSON.parse(');
    expect(discoverySource).not.toContain('response.status === 404');
    expect(discoverySource).not.toContain('return response.json()');

    expect(hostedSignupSource).toContain('parseJSONSafe<');
    expect(hostedSignupSource).not.toContain('response.json()');
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

    expect(securitySource).toContain("objectArrayFieldOrEmpty<APITokenRecord>(response, 'tokens')");
    expect(securitySource).not.toContain('response.tokens ?? []');

    expect(notificationsSource).toContain('arrayOrEmpty<Webhook>(data)');
    expect(notificationsSource).not.toContain('Array.isArray(data) ? data : []');

    expect(agentProfilesSource).toContain('arrayOrEmpty<AgentProfile>(response)');
    expect(agentProfilesSource).toContain('arrayOrEmpty<AgentProfileAssignment>(response)');
    expect(agentProfilesSource).toContain('arrayOrEmpty<ConfigKeyDefinitionResponse>(response)');
    expect(agentProfilesSource).toContain(
      "objectArrayFieldOrEmpty<ConfigValidationErrorResponse>(response, 'Errors')",
    );
    expect(agentProfilesSource).not.toContain('response || []');
    expect(agentProfilesSource).not.toContain('response.Errors || []');
  });

  it('bans raw message-based 402/404 heuristics, raw governed response-status checks, raw governed response parsing, and module-local collection fallbacks', () => {
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
      /\/(?:ai|alerts|agentProfiles|notifications|security)\.ts$/.test(path),
    );
    const rawArrayFallbackPattern = /(?:\|\||\?\?)\s*\[\]/;
    const rawArrayIsArrayFallbackPattern = /Array\.isArray\(.+\)\s*\?\s*.+:\s*\[\]/;

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
          rawArrayFallbackPattern.test(source) || rawArrayIsArrayFallbackPattern.test(source),
      )
      .map(([path]) => path)
      .sort();

    expect(heuristicOffenders).toEqual([]);
    expect(responseStatusOffenders).toEqual([]);
    expect(responseJSONOffenders).toEqual([]);
    expect(manualJSONParseOffenders).toEqual([]);
    expect(arrayFallbackOffenders).toEqual([]);
  });
});
