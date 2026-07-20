import { describe, expect, it } from 'vitest';
import {
  K8S_NAMESPACES_COLUMN_ACTIONS_LABEL,
  K8S_NAMESPACES_COLUMN_DEPLOYMENTS_LABEL,
  K8S_NAMESPACES_COLUMN_NAMESPACE_LABEL,
  K8S_NAMESPACES_COLUMN_PODS_LABEL,
  K8S_NAMESPACES_DRAWER_DESCRIPTION,
  K8S_NAMESPACES_DRAWER_TITLE,
  K8S_NAMESPACES_OPEN_ALL_PODS_LABEL,
  K8S_NAMESPACES_OPEN_PODS_LABEL,
  K8S_NAMESPACES_SEARCH_PLACEHOLDER,
  K8S_NAMESPACES_VIEW_DEPLOYMENTS_LABEL,
  getK8sNamespacesFailureState,
} from '../k8sNamespacePresentation';

// NOTE: k8sNamespacePresentation.ts is a small pure-presentation module
// (4 functions + 10 string constants). The existing sibling test
// (k8sNamespacePresentation.test.ts) already asserts the happy-path return
// value of every function: getK8sNamespacesDrawerPresentation(),
// getK8sNamespacesLoadingState(), getK8sNamespacesFailureState('boom'),
// getK8sNamespacesFailureState() (undefined), and both arms of
// getK8sNamespacesEmptyState(true|false).
//
// The module has NO namespace status/phase concept (the spec template's
// "9 uncovered getters across status/phase variants" does not map onto this
// file). The genuine residual coverage is:
//   (a) the 10 exported K8S_NAMESPACES_* constants — exercised today only
//       transitively through the function returns, never imported / asserted
//       directly;
//   (b) the falsy branch arm of `message || 'Unknown error'` in
//       getK8sNamespacesFailureState for the inputs the existing test does
//       NOT pass — namely `null` and the empty string `''` (the existing
//       test only covers a truthy string and the implicit `undefined`).
// This file targets exactly that residual.

describe('k8sNamespacePresentation.branchcov0718 — residual uncovered exports', () => {
  describe('exported label / copy constants (direct assertion)', () => {
    it('exposes the canonical drawer title and description', () => {
      expect(K8S_NAMESPACES_DRAWER_TITLE).toBe('Namespaces');
      expect(K8S_NAMESPACES_DRAWER_DESCRIPTION).toBe('Scope Pods and Deployments by namespace');
    });

    it('exposes the canonical search placeholder', () => {
      expect(K8S_NAMESPACES_SEARCH_PLACEHOLDER).toBe('Search namespaces...');
    });

    it('exposes the canonical action button labels', () => {
      expect(K8S_NAMESPACES_OPEN_ALL_PODS_LABEL).toBe('Open All Pods');
      expect(K8S_NAMESPACES_OPEN_PODS_LABEL).toBe('Open Pods');
      expect(K8S_NAMESPACES_VIEW_DEPLOYMENTS_LABEL).toBe('View Deployments');
    });

    it('exposes the canonical table column labels', () => {
      expect(K8S_NAMESPACES_COLUMN_NAMESPACE_LABEL).toBe('Namespace');
      expect(K8S_NAMESPACES_COLUMN_PODS_LABEL).toBe('Pods');
      expect(K8S_NAMESPACES_COLUMN_DEPLOYMENTS_LABEL).toBe('Deployments');
      expect(K8S_NAMESPACES_COLUMN_ACTIONS_LABEL).toBe('Actions');
    });
  });

  describe('getK8sNamespacesFailureState — residual falsy branch arms', () => {
    // Existing sibling test only passes `'boom'` (truthy) and `undefined`
    // (argument omitted). The `message || 'Unknown error'` coercion has
    // additional falsy arms that were never exercised: explicit `null` and
    // the empty string `''`. Both must fall through to the canonical
    // "Unknown error" copy.

    it('falls back to "Unknown error" when message is explicitly null', () => {
      expect(getK8sNamespacesFailureState(null)).toEqual({
        title: 'Failed to load namespaces',
        description: 'Unknown error',
      });
    });

    it('falls back to "Unknown error" when message is the empty string', () => {
      expect(getK8sNamespacesFailureState('')).toEqual({
        title: 'Failed to load namespaces',
        description: 'Unknown error',
      });
    });

    it('preserves a non-empty message and does not mutate the title', () => {
      // Guards against a regression where the `||` could be widened to `??`
      // (which would change behaviour for `''`). A non-empty message must
      // survive unchanged and the title must remain the canonical failure
      // copy regardless of the message value.
      expect(getK8sNamespacesFailureState('connection refused')).toEqual({
        title: 'Failed to load namespaces',
        description: 'connection refused',
      });
    });
  });
});
