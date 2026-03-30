import type { Resource } from '@/types/resource';
import { buildWorkloadsHrefForResource } from '@/routing/resourceLinks';

export const buildWorkloadsHref = (resource: Resource): string | null =>
  buildWorkloadsHrefForResource(resource);
