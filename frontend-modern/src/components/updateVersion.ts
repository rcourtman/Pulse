const GITHUB_RELEASES_BASE_URL = 'https://github.com/rcourtman/Pulse/releases';

export const normalizeReleaseVersion = (version?: string | null): string => {
  const trimmed = (version ?? '').trim();
  if (!trimmed) {
    return '';
  }
  return trimmed.replace(/^v/i, '');
};

export const formatReleaseTag = (version?: string | null): string => {
  const normalized = normalizeReleaseVersion(version);
  return normalized ? `v${normalized}` : '';
};

export const buildReleaseNotesUrl = (version?: string | null): string => {
  const tag = formatReleaseTag(version);
  return tag ? `${GITHUB_RELEASES_BASE_URL}/tag/${tag}` : GITHUB_RELEASES_BASE_URL;
};

export const buildDockerImageTag = (version?: string | null): string => {
  const normalized = normalizeReleaseVersion(version);
  return normalized || 'latest';
};

export const buildLinuxAmd64TarballName = (version?: string | null): string => {
  const normalized = normalizeReleaseVersion(version);
  return normalized ? `pulse-v${normalized}-linux-amd64.tar.gz` : 'pulse-linux-amd64.tar.gz';
};

export const buildLinuxAmd64DownloadCommand = (version?: string | null): string => {
  const tag = formatReleaseTag(version);
  if (!tag) {
    return '';
  }

  const tarball = buildLinuxAmd64TarballName(version);
  return (
    `curl -fL --retry 3 --retry-delay 2 -o ${tarball} ${GITHUB_RELEASES_BASE_URL}/download/${tag}/${tarball}\n` +
    `sudo tar -xzf ${tarball} -C /usr/local/bin pulse`
  );
};
