import { describe, expect, it } from 'vitest';
import { getDeployStatusPresentation } from '@/utils/deployStatusPresentation';
import type { DeployTargetStatus } from '@/types/agentDeploy';

describe('deployStatusPresentation', () => {
  it('maps each deploy target status to a canonical label', () => {
    const cases: Array<[DeployTargetStatus, string]> = [
      ['pending', 'Pending'],
      ['preflighting', 'Checking'],
      ['ready', 'Ready'],
      ['installing', 'Installing'],
      ['enrolling', 'Enrolling'],
      ['verifying', 'Verifying'],
      ['succeeded', 'Deployed'],
      ['failed_retryable', 'Failed'],
      ['failed_permanent', 'Failed'],
      ['skipped_already_agent', 'Already monitored'],
      ['skipped_license', 'License limit'],
      ['canceled', 'Canceled'],
    ];

    cases.forEach(([status, expected]) => {
      expect(getDeployStatusPresentation(status).label).toBe(expected);
    });
  });

  it('keeps progress states pulsing and failures explicit', () => {
    expect(getDeployStatusPresentation('preflighting').className).toContain('animate-pulse');
    expect(getDeployStatusPresentation('installing').className).toContain('animate-pulse');
    expect(getDeployStatusPresentation('failed_retryable').className).toContain('bg-red-100');
    expect(getDeployStatusPresentation('succeeded').className).toContain('bg-emerald-100');
  });

  it('falls back to pending for unknown or missing values', () => {
    expect(getDeployStatusPresentation(undefined)).toEqual(getDeployStatusPresentation('pending'));
    expect(getDeployStatusPresentation('bogus' as DeployTargetStatus)).toEqual(
      getDeployStatusPresentation('pending'),
    );
  });
});
