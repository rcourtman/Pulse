import { Navigate, useLocation } from '@solidjs/router';
import { buildLegacyOperationsSettingsPath } from '@/components/Settings/settingsNavigationModel';

export const Operations = () => {
  const location = useLocation();
  const canonicalPath = buildLegacyOperationsSettingsPath(location.pathname);
  return <Navigate href={`${canonicalPath}${location.search ?? ''}`} />;
};

export default Operations;
