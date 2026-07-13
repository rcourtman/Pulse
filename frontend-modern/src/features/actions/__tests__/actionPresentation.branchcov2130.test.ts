import { describe, expect, it } from 'vitest';
import type { ActionResourceReference } from '@/types/actionAudit';
import { getActionResourcePresentation } from '../actionPresentation';

describe('getActionResourcePresentation — branch coverage', () => {
  describe('resourceTypeLabel fallback for unknown resource types (L115)', () => {
    it('falls back to formatActionName when resource.type is absent from RESOURCE_TYPE_LABELS', () => {
      const result = getActionResourcePresentation('quantumdrive:0', {
        id: 'quantumdrive:0',
        name: 'My Q-Drive',
        type: 'quantumdrive',
      });
      expect(result).toEqual({ label: 'My Q-Drive', detail: 'Quantumdrive' });
    });

    it('humanizes a separator-rich unknown type via the formatActionName fallback', () => {
      const result = getActionResourcePresentation('storage:0', {
        id: 'storage:0',
        name: 'Primary Array',
        type: 'storage_array',
      });
      expect(result).toEqual({ label: 'Primary Array', detail: 'Storage Array' });
    });
  });

  describe('resource?.type ?? "" coalescing (L128)', () => {
    it('substitutes an empty string when resource.type is null, producing an empty detail', () => {
      const resource = {
        id: 'res-1',
        name: 'Unnamed Box',
        type: null as unknown as string,
      } as ActionResourceReference;
      expect(getActionResourcePresentation('res-1', resource)).toEqual({
        label: 'Unnamed Box',
        detail: '',
      });
    });

    it('substitutes an empty string when resource.type is undefined, producing an empty detail', () => {
      const resource = {
        id: 'res-2',
        name: 'Mystery Host',
        type: undefined as unknown as string,
      } as ActionResourceReference;
      expect(getActionResourcePresentation('res-2', resource)).toEqual({
        label: 'Mystery Host',
        detail: '',
      });
    });
  });

  describe('canonical kind fallback for unrecognized kind prefixes (L136)', () => {
    it('falls back to formatActionName for a single-word unknown kind', () => {
      expect(getActionResourcePresentation('terraform:workspace-1')).toEqual({
        label: 'workspace-1',
        detail: 'Terraform',
      });
    });

    it('falls back to formatActionName for a dotted unknown kind', () => {
      expect(getActionResourcePresentation('aws.ec2:i-12345678')).toEqual({
        label: 'i-12345678',
        detail: 'Aws Ec2',
      });
    });
  });

  describe('canonical opaque name compaction (L137-138 true arm)', () => {
    it('compacts a 12-char hex canonical last-part into an opaque suffix', () => {
      expect(getActionResourcePresentation('docker:container:7d3a91bd1a70')).toEqual({
        label: 'Docker container',
        detail: '…bd1a70',
      });
    });

    it('compacts a 16-char alphanumeric (non-hex) canonical last-part via the second regex arm', () => {
      expect(getActionResourcePresentation('docker:container:ghij1234567890ab')).toEqual({
        label: 'Docker container',
        detail: '…7890ab',
      });
    });

    it('uses the opaque name as detail and the kind as label when both kind and name are unrecognized', () => {
      expect(getActionResourcePresentation('terraform:7d3a91bd1a70')).toEqual({
        label: 'Terraform',
        detail: '…bd1a70',
      });
    });
  });

  describe('canonicalParts.at(-1) fallback (L134)', () => {
    it('uses the last non-empty part rather than the full normalized id, even with trailing colons', () => {
      expect(getActionResourcePresentation('docker:container:edge:')).toEqual({
        label: 'edge',
        detail: 'Docker container',
      });
    });
  });

  describe('dashed-prefix suffix opacity (L148 both arms)', () => {
    it('compacts an opaque 12-hex-char suffix on an app-container id', () => {
      expect(getActionResourcePresentation('app-container-7d3a91bd1a70')).toEqual({
        label: 'App container',
        detail: '…bd1a70',
      });
    });

    it('returns a plain (non-opaque) suffix verbatim on an app-container id', () => {
      expect(getActionResourcePresentation('app-container-edge-node')).toEqual({
        label: 'App container',
        detail: 'edge-node',
      });
    });

    it('returns a plain suffix verbatim on a vm- prefixed id', () => {
      expect(getActionResourcePresentation('vm-pve-01')).toEqual({
        label: 'Virtual machine',
        detail: 'pve-01',
      });
    });

    it('returns a plain suffix verbatim on an agent- prefixed id', () => {
      expect(getActionResourcePresentation('agent-host-01')).toEqual({
        label: 'Host agent',
        detail: 'host-01',
      });
    });
  });

  describe('OPAQUE_RESOURCE_SUFFIX boundary behavior', () => {
    it('treats an 11-char hex string as non-opaque (below the 12-char threshold)', () => {
      expect(getActionResourcePresentation('docker:container:7d3a91bd1a7')).toEqual({
        label: '7d3a91bd1a7',
        detail: 'Docker container',
      });
    });

    it('treats a 15-char alphanumeric string with non-hex letters as non-opaque', () => {
      expect(getActionResourcePresentation('docker:container:ghij12345678901')).toEqual({
        label: 'ghij12345678901',
        detail: 'Docker container',
      });
    });
  });

  describe('authoritative name guard (L124-125)', () => {
    it('falls through to the canonical path when resource.name is whitespace-only', () => {
      const resource = {
        id: 'docker:container:edge',
        name: '   ',
        type: 'container',
      } as ActionResourceReference;
      expect(getActionResourcePresentation('docker:container:edge', resource)).toEqual({
        label: 'edge',
        detail: 'Docker container',
      });
    });
  });
});
