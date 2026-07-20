import { Show, type Component, type JSX } from 'solid-js';
import { UpgradeButtonLink, type UpgradeButtonTone } from '@/components/shared/UpgradeLink';
import { UPGRADE_ACTION_LABEL } from '@/utils/upgradePresentation';
import type { UpgradeDestination } from '@/utils/upgradeNavigation';

export interface FeatureGateSectionProps {
  /** Headline naming the locked capability, e.g. "Custom Roles". */
  title: string;
  /** One-line explanation of what the capability does or why it is gated. */
  body: string;
  /** Resolved upgrade destination for this feature's key. */
  upgradeDestination: UpgradeDestination;
  /**
   * Whether to render the upgrade call-to-action. Callers pass their existing
   * gating signal (typically `!presentationPolicyHidesUpgradePrompts()`) so
   * kiosk, white-label, and presentation sessions stay action-free.
   */
  showUpgradePrompts: boolean;
  /** Optional leading icon for the capability. */
  icon?: JSX.Element;
  /** Upgrade button label. Defaults to the canonical "View plans". */
  upgradeLabel?: string;
  /** Upgrade button tone. Defaults to the primary blue treatment. */
  upgradeButtonTone?: UpgradeButtonTone;
}

/**
 * Canonical inner shell for a Pro/paid feature gate: a title and body on the
 * left, an optional leading icon, and a single upgrade call-to-action on the
 * right. Callers own the surrounding container (a divided `SettingsPanel`
 * row, a standalone `Card tone="info"`, an `OperationsPanel`, ...) so the gate
 * fits its context, while its layout, heading semantics, and upgrade button
 * stay identical across every surface that gates a feature.
 */
export const FeatureGateSection: Component<FeatureGateSectionProps> = (props) => (
  <div class="flex flex-col sm:flex-row items-center gap-4">
    <div class="flex flex-1 items-start gap-3 text-center sm:text-left">
      <Show when={props.icon}>
        <span class="mt-0.5 flex-shrink-0 text-blue-500">{props.icon}</span>
      </Show>
      <div class="flex-1">
        <h4 class="text-base font-semibold text-base-content">{props.title}</h4>
        <p class="mt-1 text-sm text-muted">{props.body}</p>
      </div>
    </div>
    <Show when={props.showUpgradePrompts}>
      <div class="flex flex-col sm:flex-row items-center gap-2">
        <UpgradeButtonLink destination={props.upgradeDestination} tone={props.upgradeButtonTone}>
          {props.upgradeLabel ?? UPGRADE_ACTION_LABEL}
        </UpgradeButtonLink>
      </div>
    </Show>
  </div>
);

export default FeatureGateSection;
