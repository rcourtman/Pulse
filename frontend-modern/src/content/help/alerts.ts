import type { HelpContent } from './types';

/**
 * Help content for alert-related features
 */
export const alertsHelpContent: HelpContent[] = [
  {
    id: 'alerts.thresholds.delay',
    title: 'Alert Delay (Sustained Duration)',
    description:
      'Click the clock icon to set per-metric delay settings.\n\n' +
      'Alert delay defines how long a threshold must be continuously exceeded ' +
      'before an alert fires. This prevents false positives from transient spikes ' +
      '(e.g., brief CPU bursts during cron jobs, backups, or container startups).\n\n' +
      'A metric must stay above the threshold for the entire delay period before alerting.',
    examples: [
      '5 seconds - Quick response, may catch brief spikes',
      '30 seconds - Balanced (recommended default)',
      '60 seconds - Conservative, filters most transient issues',
      '300 seconds - Very conservative, only sustained problems',
    ],
    addedInVersion: 'v4.0.0',
  },
  {
    id: 'alerts.thresholds.hysteresis',
    title: 'Trigger & Clear Thresholds',
    description:
      'Hysteresis prevents alert flapping when metrics hover near a threshold.\n\n' +
      'The "trigger" threshold fires the alert (e.g., CPU > 90%).\n' +
      'The "clear" threshold resolves it (e.g., CPU < 85%).\n\n' +
      'This gap prevents rapid on/off cycling when values oscillate around the threshold.',
    examples: [
      'Trigger: 90%, Clear: 85% - 5% gap prevents flapping',
      'Trigger: 95%, Clear: 90% - Tighter gap, more responsive',
    ],
    addedInVersion: 'v4.0.0',
  },
  {
    id: 'alerts.thresholds.perGuest',
    title: 'Per-Guest Threshold Overrides',
    description:
      'Each VM or container can have custom threshold settings that override the defaults.\n\n' +
      'Use this to set different thresholds for:\n' +
      '- Database servers (higher memory thresholds)\n' +
      '- Build servers (higher CPU thresholds)\n' +
      '- Development VMs (more relaxed thresholds)',
    addedInVersion: 'v4.2.0',
  },
];
