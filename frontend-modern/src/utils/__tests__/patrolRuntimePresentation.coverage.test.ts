import { describe, expect, it } from 'vitest';

import type { PatrolRuntimeState } from '@/api/patrol';

import {
  getPatrolRuntimePresentation,
  normalizePatrolRuntimeBlockedReason,
} from '@/utils/patrolRuntimePresentation';

const RETIRED_HOSTED_PATROL_BLOCKED_REASON =
  'Connect your own AI provider or local model to use Pulse Patrol.';

describe('normalizePatrolRuntimeBlockedReason', () => {
  it('returns empty string when no reason is provided', () => {
    expect(normalizePatrolRuntimeBlockedReason(undefined)).toBe('');
  });

  it('returns empty string for an empty reason', () => {
    expect(normalizePatrolRuntimeBlockedReason('')).toBe('');
  });

  it('returns empty string for a whitespace-only reason', () => {
    expect(normalizePatrolRuntimeBlockedReason('   \t\n')).toBe('');
  });

  it('rewrites a "quickstart" reason to the retired-hosted message', () => {
    expect(normalizePatrolRuntimeBlockedReason('quickstart plan expired')).toBe(
      RETIRED_HOSTED_PATROL_BLOCKED_REASON,
    );
  });

  it('rewrites a "hosted" reason to the retired-hosted message', () => {
    expect(normalizePatrolRuntimeBlockedReason('hosted plan is gone')).toBe(
      RETIRED_HOSTED_PATROL_BLOCKED_REASON,
    );
  });

  it('matches "hosted" case-insensitively', () => {
    expect(normalizePatrolRuntimeBlockedReason('HOSTED tier retired')).toBe(
      RETIRED_HOSTED_PATROL_BLOCKED_REASON,
    );
  });

  it('passes through an unrelated reason unchanged but trimmed', () => {
    expect(normalizePatrolRuntimeBlockedReason('  provider offline  ')).toBe('provider offline');
  });

  it('does not rewrite when the trigger word is a substring without word boundaries', () => {
    expect(normalizePatrolRuntimeBlockedReason('thishostedfoo')).toBe('thishostedfoo');
  });
});

describe('getPatrolRuntimePresentation', () => {
  it('maps "blocked" to a warning paused shell with a fallback description when no reason is given', () => {
    expect(getPatrolRuntimePresentation('blocked')).toMatchObject({
      label: 'Patrol paused',
      title: 'Patrol paused',
      description: 'Patrol cannot check infrastructure until the blocking condition is cleared.',
      tone: 'warning',
    });
  });

  it('uses the normalized blocked reason as the "blocked" description when provided', () => {
    expect(getPatrolRuntimePresentation('blocked', '  provider offline  ')).toMatchObject({
      label: 'Patrol paused',
      description: 'provider offline',
      tone: 'warning',
    });
  });

  it('rewrites a hosted blocked reason in the "blocked" description', () => {
    expect(getPatrolRuntimePresentation('blocked', 'hosted plan retired')).toMatchObject({
      description: RETIRED_HOSTED_PATROL_BLOCKED_REASON,
      tone: 'warning',
    });
  });

  it('ignores an empty blocked reason and falls back to the default blocked description', () => {
    expect(getPatrolRuntimePresentation('blocked', '   ').description).toBe(
      'Patrol cannot check infrastructure until the blocking condition is cleared.',
    );
  });

  it('maps "disabled" to an info disabled shell', () => {
    expect(getPatrolRuntimePresentation('disabled')).toMatchObject({
      label: 'Patrol disabled',
      title: 'Patrol disabled',
      description: 'Enable Patrol to resume checks.',
      tone: 'info',
    });
  });

  it('maps "running" to an info enabled shell with a run-in-progress title', () => {
    expect(getPatrolRuntimePresentation('running')).toMatchObject({
      label: 'Patrol enabled',
      title: 'Patrol running',
      description: 'Patrol is checking your infrastructure now.',
      tone: 'info',
    });
  });

  it('maps "unavailable" to an error unavailable shell', () => {
    expect(getPatrolRuntimePresentation('unavailable')).toMatchObject({
      label: 'Patrol unavailable',
      title: 'Patrol unavailable',
      description: 'Patrol is not ready yet. Check Provider & Models and runtime availability.',
      tone: 'error',
    });
  });

  it('maps "active" to an info enabled shell with a ready-to-check description', () => {
    expect(getPatrolRuntimePresentation('active')).toMatchObject({
      label: 'Patrol enabled',
      title: 'Patrol enabled',
      description: 'Patrol is ready to check your infrastructure.',
      tone: 'info',
    });
  });

  it('falls back to the "active" presentation for an undefined state', () => {
    expect(getPatrolRuntimePresentation(undefined)).toMatchObject({
      label: 'Patrol enabled',
      title: 'Patrol enabled',
      description: 'Patrol is ready to check your infrastructure.',
      tone: 'info',
    });
  });

  it('falls back to the "active" presentation for an unknown state value', () => {
    expect(getPatrolRuntimePresentation('idle' as unknown as PatrolRuntimeState)).toMatchObject({
      label: 'Patrol enabled',
      title: 'Patrol enabled',
      description: 'Patrol is ready to check your infrastructure.',
      tone: 'info',
    });
  });

  it('assigns a distinct tone to the warning (blocked) and error (unavailable) states', () => {
    expect(getPatrolRuntimePresentation('blocked').tone).toBe('warning');
    expect(getPatrolRuntimePresentation('unavailable').tone).toBe('error');
  });

  it('distinguishes "running" from "active" by title even though both are info/enabled', () => {
    const running = getPatrolRuntimePresentation('running');
    const active = getPatrolRuntimePresentation('active');
    expect(running.tone).toBe('info');
    expect(active.tone).toBe('info');
    expect(running.label).toBe(active.label);
    expect(running.title).not.toBe(active.title);
    expect(running.description).not.toBe(active.description);
  });

  it('gives "disabled" a label that differs from the info-enabled group', () => {
    expect(getPatrolRuntimePresentation('disabled').label).not.toBe(
      getPatrolRuntimePresentation('active').label,
    );
  });
});
