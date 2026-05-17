export const formatProxmoxVersion = (rawVersion: string | null | undefined): string => {
  const version = (rawVersion ?? '').trim();
  if (!version || version.toLowerCase() === 'unknown') return '';

  return (
    version.match(/pve-manager\/([^/\s]+)/i)?.[1] ||
    version.match(/\d+(?:\.\d+)+(?:[-+][\w.-]+)?/)?.[0] ||
    version
  );
};
