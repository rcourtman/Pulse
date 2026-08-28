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
  <div class="flex items-center gap-2.5 sm:gap-4">
    <div class="flex min-w-0 flex-1 items-start gap-2 text-left sm:gap-3">
      <Show when={props.icon}>
        <span class="mt-0.5 flex-shrink-0 text-blue-500">{props.icon}</span>
      </Show>
      <div class="min-w-0 flex-1">
        <h4 class="text-sm font-semibold text-base-content sm:text-base">{props.title}</h4>
        <p class="mt-0.5 line-clamp-2 text-[11px] leading-snug text-muted sm:mt-1 sm:text-sm sm:leading-normal">
          {props.body}
        </p>
      </div>
    </div>
    <Show when={props.showUpgradePrompts}>
      <div class="flex shrink-0 items-center gap-2">
        <UpgradeButtonLink destination={props.upgradeDestination} tone={props.upgradeButtonTone}>
          {props.upgradeLabel ?? UPGRADE_ACTION_LABEL}
        </UpgradeButtonLink>
      </div>
    </Show>
  </div>
);

export default FeatureGateSection;
