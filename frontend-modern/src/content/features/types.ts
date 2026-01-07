/**
 * Feature Tip - A discoverable feature that users might not know about
 */
export interface FeatureTip {
  /** Unique identifier for this tip */
  id: string;

  /** Short title for the tip */
  title: string;

  /** Description explaining the feature */
  description: string;

  /** Where this tip should appear in the UI */
  location: 'alerts' | 'settings' | 'docker' | 'dashboard' | 'hosts' | 'global';

  /** Version when this feature was added */
  addedInVersion: string;

  /** Optional call to action */
  action?: {
    label: string;
    path: string;
  };

  /** Priority for display order (higher = more important) */
  priority?: number;
}

