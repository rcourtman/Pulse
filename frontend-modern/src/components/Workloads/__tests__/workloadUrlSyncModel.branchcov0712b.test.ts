import { describe, expect, it } from 'vitest';

import type { WorkloadsWorkloadUrlParams } from '../workloadUrlSyncModel';
import {
  resolveWorkloadsWorkloadRuntimeParam,
  resolveWorkloadsWorkloadTypeParam,
} from '../workloadUrlSyncModel';

const params = (
  overrides: Partial<WorkloadsWorkloadUrlParams> = {},
): WorkloadsWorkloadUrlParams => ({
  type: '',
  platform: '',
  runtime: '',
  context: '',
  namespace: '',
  cluster: '',
  agent: '',
  resource: '',
  ...overrides,
});

describe('resolveWorkloadsWorkloadTypeParam (branch coverage)', () => {
  describe('normalizeWorkloadViewModeParam alias map', () => {
    it.each([
      ['all', 'all'],
      ['vm', 'vm'],
      ['container', 'container'],
      ['system-container', 'system-container'],
      ['docker', 'app-container'],
      ['app-container', 'app-container'],
      ['k8s', 'pod'],
      ['kubernetes', 'pod'],
      ['pod', 'pod'],
    ])('resolves type="%s" to "%s" when no kubernetes scope is set', (raw, expected) => {
      expect(resolveWorkloadsWorkloadTypeParam(params({ type: raw }))).toBe(expected);
    });

    it('trims and lowercases the raw type before alias resolution', () => {
      expect(resolveWorkloadsWorkloadTypeParam(params({ type: '  VM  ' }))).toBe('vm');
      expect(resolveWorkloadsWorkloadTypeParam(params({ type: 'KUBERNETES' }))).toBe('pod');
    });
  });

  describe('ungoverned type → null (normalizeWorkloadViewModeParam fall-through)', () => {
    it('returns null when type is empty', () => {
      expect(resolveWorkloadsWorkloadTypeParam(params({ type: '' }))).toBeNull();
    });

    it('returns null when type matches no known alias', () => {
      expect(resolveWorkloadsWorkloadTypeParam(params({ type: 'spaceship' }))).toBeNull();
    });
  });

  describe('kubernetes scope precedence (hasWorkloadsWorkloadKubernetesScope)', () => {
    it('suppresses a non-pod mode when only context is set', () => {
      expect(resolveWorkloadsWorkloadTypeParam(params({ type: 'vm', context: 'prod' }))).toBeNull();
    });

    it('suppresses a non-pod mode when only namespace is set', () => {
      expect(
        resolveWorkloadsWorkloadTypeParam(params({ type: 'docker', namespace: 'default' })),
      ).toBeNull();
    });

    it('suppresses a non-pod mode when both context and namespace are set', () => {
      expect(
        resolveWorkloadsWorkloadTypeParam(
          params({ type: 'all', context: 'prod', namespace: 'default' }),
        ),
      ).toBeNull();
    });

    it('keeps the pod mode when context is set (nextMode === "pod")', () => {
      expect(resolveWorkloadsWorkloadTypeParam(params({ type: 'pod', context: 'prod' }))).toBe(
        'pod',
      );
    });

    it('keeps the pod mode when namespace is set and the raw alias is "k8s"', () => {
      expect(
        resolveWorkloadsWorkloadTypeParam(params({ type: 'k8s', namespace: 'default' })),
      ).toBe('pod');
    });

    it('treats whitespace-only context/namespace as no scope', () => {
      // Boolean(" ".trim()) === false for both fields, so the precedence guard
      // is skipped and the resolved mode is returned unchanged.
      expect(
        resolveWorkloadsWorkloadTypeParam(
          params({ type: 'vm', context: '   ', namespace: '   ' }),
        ),
      ).toBe('vm');
    });
  });
});

describe('resolveWorkloadsWorkloadRuntimeParam (branch coverage)', () => {
  describe('Branch 1 — runtimeRelevant=false (passthrough)', () => {
    it('is not relevant when nextMode is a non-container view (ternary true-arm, isContainer=false)', () => {
      // type="vm": nextMode="vm", isContainerWorkloadViewMode(vm)=false,
      // params.type non-empty → disjunction false → runtimeRelevant=false.
      expect(
        resolveWorkloadsWorkloadRuntimeParam(params({ type: 'vm', runtime: 'containerd' })),
      ).toStrictEqual({ forceViewMode: null, runtime: 'containerd', shouldApply: false });
    });

    it('is not relevant when nextMode is pod', () => {
      expect(
        resolveWorkloadsWorkloadRuntimeParam(params({ type: 'pod', runtime: 'containerd' })),
      ).toStrictEqual({ forceViewMode: null, runtime: 'containerd', shouldApply: false });
    });

    it('is not relevant when type is unknown and non-empty (ternary false-arm of nextMode check)', () => {
      // type="spaceship": nextMode=null → `(nextMode ? ... : false)` evaluates
      // its false arm; combined with non-empty params.type the disjunction is false.
      expect(
        resolveWorkloadsWorkloadRuntimeParam(params({ type: 'spaceship', runtime: 'podman' })),
      ).toStrictEqual({ forceViewMode: null, runtime: 'podman', shouldApply: false });
    });

    it('is not relevant when kubernetes scope is present, even for a container type', () => {
      // !hasWorkloadsWorkloadKubernetesScope short-circuits the entire expression.
      expect(
        resolveWorkloadsWorkloadRuntimeParam(
          params({ type: 'docker', runtime: 'containerd', context: 'prod' }),
        ),
      ).toStrictEqual({ forceViewMode: null, runtime: 'containerd', shouldApply: false });
    });

    it('is not relevant when kubernetes scope is set via namespace only', () => {
      expect(
        resolveWorkloadsWorkloadRuntimeParam(
          params({ type: 'docker', runtime: 'containerd', namespace: 'kube-system' }),
        ),
      ).toStrictEqual({ forceViewMode: null, runtime: 'containerd', shouldApply: false });
    });
  });

  describe('Branch 2 — runtimeRelevant=true, empty runtime', () => {
    it('clears runtime to "" when runtime is empty for a container type', () => {
      expect(
        resolveWorkloadsWorkloadRuntimeParam(params({ type: 'docker', runtime: '' })),
      ).toStrictEqual({ forceViewMode: null, runtime: '', shouldApply: true });
    });

    it('treats whitespace-only runtime as empty (trim branch) and emits runtime=""', () => {
      // Branch 2 returns the literal "" — NOT params.runtime. Asserts that
      // whitespace-only input is normalized via the `.trim()` guard.
      expect(
        resolveWorkloadsWorkloadRuntimeParam(params({ type: 'docker', runtime: '   ' })),
      ).toStrictEqual({ forceViewMode: null, runtime: '', shouldApply: true });
    });

    it('reaches Branch 2 via the empty-type relevance path (!params.type.trim())', () => {
      // type="": nextMode=null and the ternary returns false, but the
      // `|| !params.type.trim()` disjunct flips runtimeRelevant to true.
      expect(
        resolveWorkloadsWorkloadRuntimeParam(params({ type: '', runtime: '' })),
      ).toStrictEqual({ forceViewMode: null, runtime: '', shouldApply: true });
    });
  });

  describe('Branch 3 — runtimeRelevant=true, present runtime (forceViewMode="container")', () => {
    it('forces container view via the empty-type path when runtime is present', () => {
      // type="" makes nextMode=null but the empty-type disjunct keeps relevance;
      // a non-empty runtime then bypasses Branch 2.
      expect(
        resolveWorkloadsWorkloadRuntimeParam(params({ type: '', runtime: 'podman' })),
      ).toStrictEqual({ forceViewMode: 'container', runtime: 'podman', shouldApply: true });
    });

    it('passes a whitespace-padded runtime through verbatim in Branch 3', () => {
      // OBSERVATION (reported, not fixed): Branch 3 returns `runtime: params.runtime`
      // untrimmed, while Branch 2 normalizes whitespace-only to "". The two arms
      // trim inconsistently.
      expect(
        resolveWorkloadsWorkloadRuntimeParam(params({ type: 'docker', runtime: '  podman  ' })),
      ).toStrictEqual({ forceViewMode: 'container', runtime: '  podman  ', shouldApply: true });
    });
  });
});
