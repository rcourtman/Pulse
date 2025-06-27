"use client";

import { useQuery } from "@tanstack/react-query";

import type { AppVersion } from "@/lib/types";

import { fetchVersions } from "@/lib/data";

export function useVersions() {
  return useQuery<AppVersion[]>({
    queryKey: ["versions"],
    queryFn: async () => {
      const fetchedVersions = await fetchVersions();
      if (Array.isArray(fetchedVersions)) {
        return fetchedVersions;
      }
      if (fetchedVersions && typeof fetchedVersions === "object") {
        return [fetchedVersions];
      }
      return [];
    },
  });
}
