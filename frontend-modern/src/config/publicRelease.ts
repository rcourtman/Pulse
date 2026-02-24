export type PublicReleaseTrack = 'v5' | 'v6';

export function normalizePublicReleaseTrack(value: string | null | undefined): PublicReleaseTrack {
  const normalized = (value ?? '').trim().toLowerCase();
  return normalized === 'v6' ? 'v6' : 'v5';
}

// Build-time switch (default v5): set VITE_PUBLIC_RELEASE_TRACK=v6 to expose v6 public pricing.
export const PUBLIC_RELEASE_TRACK: PublicReleaseTrack = normalizePublicReleaseTrack(
  import.meta.env.VITE_PUBLIC_RELEASE_TRACK,
);

export const IS_V6_PUBLIC_RELEASE = PUBLIC_RELEASE_TRACK === 'v6';
