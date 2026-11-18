import { createSessionActivityRecorder } from './sessionActivity';

export type InfrastructureMapActivity = 'open' | 'export' | 'export_names_hidden';

const recorder = createSessionActivityRecorder<InfrastructureMapActivity>({
  endpoint: '/api/usage/infrastructure-map',
  storageKeyPrefix: 'pulse:infrastructure-map-activity:v1:',
});

/**
 * Records that the map page was opened, at most once per browser session.
 * The server keeps only a daily count. Nothing the map draws is sent.
 */
export const recordInfrastructureMapOpen = (): void => recorder.record('open');

/**
 * Records that a map image was saved, and separately that one was saved with
 * names hidden, each at most once per browser session. The image itself and
 * everything in it stay in the browser.
 */
export const recordInfrastructureMapExport = (options: { namesHidden: boolean }): void => {
  recorder.record('export');
  if (options.namesHidden) recorder.record('export_names_hidden');
};

export const resetInfrastructureMapActivityForTest = (): void => recorder.resetForTest();
