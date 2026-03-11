import { describe, expect, it } from 'vitest';
import aiSource from '@/api/ai.ts?raw';
import agentProfilesSource from '@/api/agentProfiles.ts?raw';
import discoverySource from '@/api/discovery.ts?raw';
import monitoringSource from '@/api/monitoring.ts?raw';
import responseUtilsSource from '@/api/responseUtils.ts?raw';

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
    expect(responseUtilsSource).toContain('(error as APIErrorLike).status');
    expect(responseUtilsSource).toContain('response.status');
  });

  it('routes canonical paywall and not-found API error handling through responseUtils', () => {
    expect(aiSource).toContain('isAPIErrorStatus(error, 402)');
    expect(aiSource).not.toContain("message.includes('402')");

    expect(agentProfilesSource).toContain('isAPIErrorStatus(err, 402)');
    expect(agentProfilesSource).toContain('isAPIErrorStatus(err, 404)');
    expect(agentProfilesSource).not.toContain("message.includes('402')");
    expect(agentProfilesSource).not.toContain("message.includes('404')");

    expect(monitoringSource).toContain('isAPIResponseStatus(response, 404)');
    expect(discoverySource).toContain('isAPIResponseStatus(response, 404)');
    expect(monitoringSource).not.toContain('response.status === 404');
    expect(discoverySource).not.toContain('response.status === 404');
  });

  it('bans raw message-based 402/404 heuristics and raw 404 response checks in runtime API modules', () => {
    const runtimeEntries = Object.entries(apiSources).filter(
      ([path]) => !path.endsWith('/responseUtils.ts'),
    );
    const rawStatusHeuristicPattern = /message\.includes\((['"])40[24]\1\)/;
    const rawResponse404Pattern = /response\.status === 404/;

    const heuristicOffenders = runtimeEntries
      .filter(([, source]) => rawStatusHeuristicPattern.test(source))
      .map(([path]) => path)
      .sort();

    const response404Offenders = runtimeEntries
      .filter(([, source]) => rawResponse404Pattern.test(source))
      .map(([path]) => path)
      .sort();

    expect(heuristicOffenders).toEqual([]);
    expect(response404Offenders).toEqual([]);
  });
});
