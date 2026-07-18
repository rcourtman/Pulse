import { describe, expect, it } from 'vitest';
import {
  ALERT_THRESHOLDS_DOCKER_UPDATES_TITLE,
  ALERT_THRESHOLDS_DOCKER_UPDATES_DESCRIPTION,
  ALERT_THRESHOLDS_DOCKER_UPDATES_TOGGLE_LABEL,
  ALERT_THRESHOLDS_DOCKER_UPDATES_TOGGLE_DESCRIPTION,
  ALERT_THRESHOLDS_DOCKER_UPDATES_DELAY_LABEL,
  ALERT_THRESHOLDS_DOCKER_UPDATES_DELAY_DESCRIPTION,
  getAlertThresholdsDockerUpdatePresentation,
} from '../alertThresholdsPresentation';

describe('alertThresholdsPresentation.branchcov0718', () => {
  it('wires getAlertThresholdsDockerUpdatePresentation to the canonical DOCKER_UPDATES constants', () => {
    expect(getAlertThresholdsDockerUpdatePresentation()).toEqual({
      title: ALERT_THRESHOLDS_DOCKER_UPDATES_TITLE,
      description: ALERT_THRESHOLDS_DOCKER_UPDATES_DESCRIPTION,
      toggleLabel: ALERT_THRESHOLDS_DOCKER_UPDATES_TOGGLE_LABEL,
      toggleDescription: ALERT_THRESHOLDS_DOCKER_UPDATES_TOGGLE_DESCRIPTION,
      delayLabel: ALERT_THRESHOLDS_DOCKER_UPDATES_DELAY_LABEL,
      delayDescription: ALERT_THRESHOLDS_DOCKER_UPDATES_DELAY_DESCRIPTION,
    });
  });
});
