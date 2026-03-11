import { describe, expect, it } from 'vitest';
import {
  getDiscoveryInitialEmptyState,
  getDiscoveryLoadingState,
  getDiscoveryNotesEmptyState,
  getDiscoveryAnalysisProviderBadgeClass,
  getDiscoveryCategoryBadgeClass,
  getDiscoverySuggestedURLFallback,
  getDiscoveryURLSuggestionSourceLabel,
} from '@/utils/discoveryPresentation';

describe('discoveryPresentation', () => {
  it('returns canonical proxmox profile labels', () => {
    expect(getDiscoveryURLSuggestionSourceLabel('host_management_profile_pve')).toBe(
      'Proxmox node profile',
    );
    expect(getDiscoveryURLSuggestionSourceLabel('host_management_profile_pbs')).toBe(
      'Proxmox Backup profile',
    );
    expect(getDiscoveryURLSuggestionSourceLabel('host_management_profile_pmg')).toBe(
      'Proxmox Mail Gateway profile',
    );
  });

  it('falls back to the generic discovery heuristic label', () => {
    expect(getDiscoveryURLSuggestionSourceLabel('')).toBe('Discovery heuristic');
    expect(getDiscoveryURLSuggestionSourceLabel('unknown-code')).toBe('Discovery heuristic');
  });

  it('returns canonical analysis provider badge classes', () => {
    expect(getDiscoveryAnalysisProviderBadgeClass(true)).toContain('bg-green-100');
    expect(getDiscoveryAnalysisProviderBadgeClass(false)).toContain('bg-blue-100');
  });

  it('returns a canonical discovery category badge class', () => {
    expect(getDiscoveryCategoryBadgeClass()).toContain('bg-blue-100');
    expect(getDiscoveryCategoryBadgeClass()).toContain('text-blue-700');
  });

  it('returns canonical discovery empty-state copy', () => {
    expect(getDiscoveryInitialEmptyState(true)).toEqual({
      title: 'Checking existing discovery data...',
      description: 'You can run discovery now if this takes too long.',
    });
    expect(getDiscoveryInitialEmptyState(false)).toEqual({
      title: 'No discovery data yet',
      description: 'Run a discovery scan to identify services and configurations',
    });
    expect(getDiscoverySuggestedURLFallback('diag')).toEqual({
      title: 'No suggested URL found',
      description: 'diag',
    });
    expect(getDiscoveryLoadingState()).toEqual({
      text: 'Loading discovery...',
    });
    expect(getDiscoveryNotesEmptyState()).toEqual({
      text: 'No notes yet. Add notes to document important information.',
    });
  });
});
