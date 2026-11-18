import { layoutServiceDiagram } from './infrastructureMapDiagram';
import {
  layoutInfrastructureMap,
  layoutInfrastructureNetworkMap,
  type MapDensity,
  type MapLayoutOptions,
  type MapView,
} from './infrastructureMapLayout';
import { formatMapTotalLabel, type InfrastructureMapModel } from './infrastructureMapModel';
import { renderMapSceneToSvg, type MapPalette } from './infrastructureMapRender';
import { buildServiceDiagram, DIAGRAM_MAX_TILES_PER_GROUP } from './infrastructureMapServices';

/**
 * The shareable image: the same model the page draws, laid out at a poster
 * width of its own with a title above it. Nothing leaves the browser; the
 * file is built and saved locally.
 */

const EXPORT_COLUMN = 300;
// The service diagram reads best wide: its boxes sit side by side in a row.
const EXPORT_DIAGRAM_WIDTH = 1480;
const EXPORT_SCALE = 2;
// Browsers refuse canvases past roughly 16k pixels a side; beyond that the
// image is still saved, as the SVG it was drawn from.
const MAX_CANVAS_SIDE = 16000;

export interface MapExportOptions {
  title: string;
  view: MapView;
  density: MapDensity;
  palette: MapPalette;
  now?: Date;
}

const countCards = (model: InfrastructureMapModel): number =>
  model.blocks.reduce(
    (sum, block) => sum + (block.kind === 'group' ? block.group.hosts.length : 1),
    0,
  );

/** A width that lands near a 4:3 poster for the number of cards drawn. */
export const resolveMapExportWidth = (model: InfrastructureMapModel): number => {
  const columns = Math.min(6, Math.max(2, Math.round(Math.sqrt(countCards(model) * 1.6))));
  return columns * EXPORT_COLUMN + 56;
};

/** Small tiles hold twice as many to a box before the rest fold into one. */
export const resolveDiagramTileLimit = (density: MapDensity): number =>
  density === 'marks' ? DIAGRAM_MAX_TILES_PER_GROUP * 2 : DIAGRAM_MAX_TILES_PER_GROUP;

export function buildMapExportSvg(
  model: InfrastructureMapModel,
  options: MapExportOptions,
): { svg: string; width: number; height: number } {
  const date = (options.now ?? new Date()).toISOString().slice(0, 10);
  const layoutOptions: MapLayoutOptions = {
    width: options.view === 'services' ? EXPORT_DIAGRAM_WIDTH : resolveMapExportWidth(model),
    density: options.density,
    header: {
      title: options.title,
      facts: model.totals.map(
        (total) => `${total.count.toLocaleString()} ${formatMapTotalLabel(total)}`,
      ),
      footer: `Drawn by Pulse · ${date}`,
    },
  };
  const scene =
    options.view === 'services'
      ? layoutServiceDiagram(
          buildServiceDiagram(model, {
            maxTilesPerGroup: resolveDiagramTileLimit(options.density),
          }),
          layoutOptions,
        )
      : options.view === 'networks'
        ? layoutInfrastructureNetworkMap(model, layoutOptions)
        : layoutInfrastructureMap(model, layoutOptions);
  return {
    svg: renderMapSceneToSvg(scene, options.palette),
    width: scene.width,
    height: scene.height,
  };
}

const saveBlob = (blob: Blob, filename: string): void => {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  // The download reads the URL after the click returns.
  window.setTimeout(() => URL.revokeObjectURL(url), 10_000);
};

const svgToPngBlob = (svg: string, width: number, height: number): Promise<Blob> =>
  new Promise((resolve, reject) => {
    const url = URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml;charset=utf-8' }));
    const image = new Image();
    image.onload = () => {
      URL.revokeObjectURL(url);
      const canvas = document.createElement('canvas');
      canvas.width = width * EXPORT_SCALE;
      canvas.height = height * EXPORT_SCALE;
      const context = canvas.getContext('2d');
      if (!context) {
        reject(new Error('No drawing context'));
        return;
      }
      context.drawImage(image, 0, 0, canvas.width, canvas.height);
      canvas.toBlob(
        (blob) => (blob ? resolve(blob) : reject(new Error('Image encoding failed'))),
        'image/png',
      );
    };
    image.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error('Image rendering failed'));
    };
    image.src = url;
  });

export type MapExportFormat = 'png' | 'svg';

/** Saves the map as an image and reports which format was written. */
export async function downloadMapImage(
  model: InfrastructureMapModel,
  options: MapExportOptions,
): Promise<MapExportFormat> {
  const { svg, width, height } = buildMapExportSvg(model, options);
  const date = (options.now ?? new Date()).toISOString().slice(0, 10);
  const basename = `pulse-map-${date}`;
  const fits = width * EXPORT_SCALE <= MAX_CANVAS_SIDE && height * EXPORT_SCALE <= MAX_CANVAS_SIDE;
  if (fits) {
    try {
      saveBlob(await svgToPngBlob(svg, width, height), `${basename}.png`);
      return 'png';
    } catch {
      // Fall through: the SVG is the same picture and always saves.
    }
  }
  saveBlob(new Blob([svg], { type: 'image/svg+xml;charset=utf-8' }), `${basename}.svg`);
  return 'svg';
}
