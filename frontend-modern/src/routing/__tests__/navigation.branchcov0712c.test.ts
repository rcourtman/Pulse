import { describe, expect, it } from 'vitest';
import { getActiveTabForPath } from '@/routing/navigation';
import {
  ACTIONS_PATH,
  DOCKER_PATH,
  KUBERNETES_PATH,
  PROXMOX_PATH,
  STANDALONE_PATH,
  TRUENAS_PATH,
  VMWARE_PATH,
} from '@/routing/resourceLinks';

// Branch-coverage companion to navigation.test.ts. The sibling suite drives the
// proxmox / standalone / alerts / actions / ai (patrol) / settings true-arms and
// the `return null` fallback, but never forces the docker, kubernetes, truenas,
// or vmware `if (path.startsWith(...))` checks to a truthy return. Each block
// below targets one of those four uncovered true-arms (plus the empty-path
// fallback and the no-boundary `startsWith` over-match documented in
// GLM_REPORT.md).

describe('getActiveTabForPath branch coverage', () => {
  it('returns "docker" for the DOCKER_PATH root and nested docker routes (docker true-arm)', () => {
    expect(getActiveTabForPath(DOCKER_PATH)).toBe('docker');
    expect(getActiveTabForPath(`${DOCKER_PATH}/containers`)).toBe('docker');
    expect(getActiveTabForPath(`${DOCKER_PATH}/host/pve-1`)).toBe('docker');
  });

  it('returns "kubernetes" for the KUBERNETES_PATH root and nested routes (kubernetes true-arm)', () => {
    expect(getActiveTabForPath(KUBERNETES_PATH)).toBe('kubernetes');
    expect(getActiveTabForPath(`${KUBERNETES_PATH}/workloads`)).toBe('kubernetes');
    expect(getActiveTabForPath(`${KUBERNETES_PATH}/config/networking`)).toBe('kubernetes');
  });

  it('returns "truenas" for the TRUENAS_PATH root and nested routes (truenas true-arm)', () => {
    expect(getActiveTabForPath(TRUENAS_PATH)).toBe('truenas');
    expect(getActiveTabForPath(`${TRUENAS_PATH}/datasets`)).toBe('truenas');
    expect(getActiveTabForPath(`${TRUENAS_PATH}/sharing/smb`)).toBe('truenas');
  });

  it('returns "vmware" for the VMWARE_PATH root and nested routes (vmware true-arm)', () => {
    expect(getActiveTabForPath(VMWARE_PATH)).toBe('vmware');
    expect(getActiveTabForPath(`${VMWARE_PATH}/hosts`)).toBe('vmware');
    expect(getActiveTabForPath(`${VMWARE_PATH}/hosts/clusters`)).toBe('vmware');
  });

  it('returns null for an empty path so every startsWith false-arm runs in sequence', () => {
    expect(getActiveTabForPath('')).toBeNull();
  });

  it('returns null for single-character slashes that share only the leading "/"', () => {
    expect(getActiveTabForPath('/d')).toBeNull();
    expect(getActiveTabForPath('/k')).toBeNull();
    expect(getActiveTabForPath('/t')).toBeNull();
    expect(getActiveTabForPath('/v')).toBeNull();
  });

  it('over-matches because startsWith enforces no "/" boundary after the prefix', () => {
    // Documents current semantics: a path that merely begins with the literal
    // prefix — with no path-segment boundary — still resolves to the tab.
    // Reported as a suspected over-match in GLM_REPORT.md (not fixed here).
    expect(getActiveTabForPath(`${DOCKER_PATH}x`)).toBe('docker');
    expect(getActiveTabForPath(`${KUBERNETES_PATH}x`)).toBe('kubernetes');
    expect(getActiveTabForPath(`${TRUENAS_PATH}x`)).toBe('truenas');
    expect(getActiveTabForPath(`${VMWARE_PATH}x`)).toBe('vmware');
    expect(getActiveTabForPath(`${PROXMOX_PATH}y`)).toBe('proxmox');
    expect(getActiveTabForPath(`${STANDALONE_PATH}y`)).toBe('standalone');
    expect(getActiveTabForPath(`${ACTIONS_PATH}y`)).toBe('actions');
  });

  it('resolves each platform root to its own tab rather than a later arm (precedence sanity)', () => {
    // The docker/kubernetes/truenas/vmware checks run before standalone and the
    // remaining arms; confirm a platform root is classified by its own check.
    expect(getActiveTabForPath(DOCKER_PATH)).toBe('docker');
    expect(getActiveTabForPath(KUBERNETES_PATH)).toBe('kubernetes');
    expect(getActiveTabForPath(TRUENAS_PATH)).toBe('truenas');
    expect(getActiveTabForPath(VMWARE_PATH)).toBe('vmware');
  });
});
