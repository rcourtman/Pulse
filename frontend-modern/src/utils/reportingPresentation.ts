export function getReportingGenerateSelectionRequiredMessage(): string {
  return 'Please select at least one resource';
}

export function getReportingGenerateSuccessMessage(): string {
  return 'Report generated successfully';
}

export function getReportingGenerateErrorMessage(): string {
  return 'Failed to generate report';
}

export function getReportingCatalogErrorMessage(): string {
  return 'Failed to load reporting surfaces';
}

export function getReportingInventoryExportSuccessMessage(): string {
  return 'VM inventory export generated successfully';
}

export function getReportingInventoryExportErrorMessage(): string {
  return 'Failed to generate VM inventory export';
}

function decodeRFC5987Value(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

export function resolveReportingDownloadFilename(
  contentDisposition: string | null,
  fallbackFilename: string,
): string {
  if (typeof contentDisposition !== 'string' || contentDisposition.trim() === '') {
    return fallbackFilename;
  }

  const encodedMatch = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (encodedMatch && encodedMatch[1]) {
    return decodeRFC5987Value(encodedMatch[1].trim());
  }

  const quotedMatch = contentDisposition.match(/filename="([^"]+)"/i);
  if (quotedMatch && quotedMatch[1]) {
    return quotedMatch[1];
  }

  const plainMatch = contentDisposition.match(/filename=([^;]+)/i);
  if (plainMatch && plainMatch[1]) {
    return plainMatch[1].trim();
  }

  return fallbackFilename;
}
