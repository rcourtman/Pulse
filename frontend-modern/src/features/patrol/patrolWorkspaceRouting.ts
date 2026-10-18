import { PATROL_PATH } from '@/routing/resourceLinks';

export type PatrolWorkspaceView = 'inbox' | 'protection' | 'activity';

export const PATROL_WORKSPACE_VIEWS: readonly PatrolWorkspaceView[] = [
  'inbox',
  'protection',
  'activity',
];

// Each workspace view is a route (/patrol, /patrol/protection,
// /patrol/activity), so a bookmark or shared link opens the view it was copied
// from and back/forward moves between them.
export function buildPatrolWorkspacePath(view: PatrolWorkspaceView): string {
  return view === 'inbox' ? PATROL_PATH : `${PATROL_PATH}/${view}`;
}

export function parsePatrolWorkspaceView(pathname: string): PatrolWorkspaceView {
  const match = /^\/patrol\/(protection|activity)\/?$/.exec(pathname);
  return match ? (match[1] as PatrolWorkspaceView) : 'inbox';
}
