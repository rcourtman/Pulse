import { createSessionActivityRecorder } from './sessionActivity';

export type InfrastructureMapActivity =
  | 'open'
  | 'export'
  | 'export_services'
  | 'export_connections'
  | 'export_hosts'
  | 'export_names_hidden';

export type InfrastructureMapExportView = 'services' | 'networks' | 'hosts';

// A view added to the map must be added here, or its saves have no view count.
const EXPORT_VIEW_ACTIVITY: Record<InfrastructureMapExportView, InfrastructureMapActivity> = {
  services: 'export_services',
  networks: 'export_connections',
  hosts: 'export_hosts',
};

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
 * Records that a map image was saved, at most once per browser session for
 * each fact: that an image was saved, which view it showed, and whether names
 * were hidden. The image itself and everything in it stay in the browser.
 */
export const recordInfrastructureMapExport = (options: {
  view: InfrastructureMapExportView;
  namesHidden: boolean;
}): void => {
  recorder.record('export');
  recorder.record(EXPORT_VIEW_ACTIVITY[options.view]);
  if (options.namesHidden) recorder.record('export_names_hidden');
};

export const resetInfrastructureMapActivityForTest = (): void => recorder.resetForTest();
