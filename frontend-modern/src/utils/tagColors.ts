interface TagColorStyle {
  bg: string;
  text: string;
  border: string;
}

function stringToRGB(tag: string): [number, number, number] {
  let hash = 0;
  if (!tag) {
    return [255, 255, 255];
  }

  const value = `${tag.toLowerCase()}prox`;
  for (let i = 0; i < value.length; i++) {
    hash = value.charCodeAt(i) + ((hash << 5) - hash);
    hash &= hash;
  }

  const alpha = 0.7;
  const bg = 255;

  return [
    (hash & 255) * alpha + bg * (1 - alpha),
    ((hash >> 8) & 255) * alpha + bg * (1 - alpha),
    ((hash >> 16) & 255) * alpha + bg * (1 - alpha),
  ];
}

function rgbToCss(rgb: [number, number, number]): string {
  return `rgb(${rgb[0]}, ${rgb[1]}, ${rgb[2]})`;
}

function getTextContrastClass(rgb: [number, number, number]): 'light' | 'dark' {
  const blkThrs = 0.022;
  const blkClmp = 1.414;

  const r = (rgb[0] / 255) ** 2.4;
  const g = (rgb[1] / 255) ** 2.4;
  const b = (rgb[2] / 255) ** 2.4;

  let bg = r * 0.2126729 + g * 0.7151522 + b * 0.072175;
  bg = bg > blkThrs ? bg : bg + (blkThrs - bg) ** blkClmp;

  const contrastLight = bg ** 0.65 - 1;
  const contrastDark = bg ** 0.56 - 0.046134502;

  return Math.abs(contrastLight) >= Math.abs(contrastDark) ? 'light' : 'dark';
}

function parseHexColor(hex: string): [number, number, number] | null {
  const normalized = hex.trim().replace(/^#/, '');
  if (!/^[0-9a-fA-F]{6}$/.test(normalized)) {
    return null;
  }

  return [
    parseInt(normalized.slice(0, 2), 16),
    parseInt(normalized.slice(2, 4), 16),
    parseInt(normalized.slice(4, 6), 16),
  ];
}

function buildStyleFromRGB(rgb: [number, number, number]): TagColorStyle {
  const bg = rgbToCss(rgb);
  const text = getTextContrastClass(rgb) === 'light' ? '#ffffff' : '#000000';

  return {
    bg,
    text,
    border: bg,
  };
}

/**
 * Get color for a tag.
 * Priority: Proxmox-supplied hex color -> Proxmox fallback hash algorithm.
 */
export function getTagColorWithSpecial(
  tag: string,
  _isDarkMode: boolean,
  colorMap?: Record<string, string>,
): TagColorStyle {
  const lowerTag = tag.toLowerCase();
  const proxmoxHex = colorMap?.[lowerTag];
  if (proxmoxHex) {
    const rgb = parseHexColor(proxmoxHex);
    if (rgb) {
      return buildStyleFromRGB(rgb);
    }
  }

  return buildStyleFromRGB(stringToRGB(tag));
}
