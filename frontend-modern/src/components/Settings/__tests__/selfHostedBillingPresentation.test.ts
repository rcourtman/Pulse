import { describe, expect, it } from 'vitest';
import {
  infrastructureLooksProviderSized,
  type InfrastructureMspReferralRow,
} from '../selfHostedBillingPresentation';

const rowsOf = (
  ownerType: string,
  count: number,
  isAgent = false,
): InfrastructureMspReferralRow[] => Array.from({ length: count }, () => ({ ownerType, isAgent }));

// The thresholds are written out here on purpose: the contract promises
// three Proxmox VE or Backup Server connections or 25 agents, so a change to
// the shipped constants must fail this file.
describe('infrastructureLooksProviderSized', () => {
  it('leaves a home lab with a couple of clusters and backup servers alone', () => {
    expect(infrastructureLooksProviderSized([])).toBe(false);
    expect(
      infrastructureLooksProviderSized([
        ...rowsOf('pve', 2),
        ...rowsOf('pbs', 2),
        ...rowsOf('pmg', 5),
        ...rowsOf('agent', 24, true),
      ]),
    ).toBe(false);
  });

  it('counts three separate Proxmox VE or Backup Server connections', () => {
    expect(infrastructureLooksProviderSized(rowsOf('pve', 2))).toBe(false);
    expect(infrastructureLooksProviderSized(rowsOf('pve', 3))).toBe(true);
    expect(infrastructureLooksProviderSized(rowsOf('pbs', 2))).toBe(false);
    expect(infrastructureLooksProviderSized(rowsOf('pbs', 3))).toBe(true);
  });

  it('counts 25 agents, and never an agent row as a Proxmox connection', () => {
    expect(infrastructureLooksProviderSized(rowsOf('agent', 24, true))).toBe(false);
    expect(infrastructureLooksProviderSized(rowsOf('agent', 25, true))).toBe(true);
    expect(infrastructureLooksProviderSized(rowsOf('pve', 3, true))).toBe(false);
  });
});
