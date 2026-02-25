import type { HelpContent } from './types';

/**
 * Help content for update-related features
 */
export const updatesHelpContent: HelpContent[] = [
  {
    id: 'updates.docker.notifications',
    title: 'Container Update Detection',
    description:
      'Pulse automatically detects when newer versions of your container images are available.\n\n' +
      'How it works:\n' +
      '- Compares local image digests against registry manifests\n' +
      '- Checks periodically without pulling full images\n' +
      '- Shows update badges on containers with available updates\n\n' +
      'Supported registries: Docker Hub, GitHub Container Registry (ghcr.io), ' +
      'and most private registries with v2 API support.',
    examples: [
      'Blue badge = Update available',
      'Hover the badge for version details',
      'Click container row for update instructions',
    ],
    addedInVersion: 'v3.0.0',
  },
  {
    id: 'updates.pulse.channel',
    title: 'Update Channel',
    description:
      'Choose which Pulse releases to be notified about:\n\n' +
      '- Stable: Production-ready releases only\n' +
      '- Release Candidate (RC): Preview upcoming features before stable release\n\n' +
      'RC builds are tested but may have rough edges. Use stable for production environments.\n\n' +
      'For major version pre-releases (e.g. v6.0.0-rc.1), we strongly recommend installing ' +
      'as a separate instance rather than upgrading your production installation.',
    examples: ['stable - v5.0.11, v5.0.12, etc.', 'rc - v5.1.0-rc.1, v6.0.0-rc.1, etc.'],
    addedInVersion: 'v4.0.0',
  },
  {
    id: 'updates.docker.checkInterval',
    title: 'Docker Update Check Interval',
    description:
      'How often Pulse checks for container image updates.\n\n' +
      'More frequent checks catch updates sooner but increase registry API calls. ' +
      'Most registries have generous rate limits, but very frequent checks on many ' +
      'containers could hit limits.',
    examples: [
      '1 hour - Frequent updates, higher API usage',
      '6 hours - Balanced (recommended)',
      '24 hours - Conservative, minimal API usage',
    ],
    addedInVersion: 'v4.0.0',
  },
];
