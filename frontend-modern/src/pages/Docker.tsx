import { Navigate, useLocation } from '@solidjs/router';
import { DockerPageSurface } from '@/features/docker/DockerPageSurface';
import { buildDockerPath, DOCKER_PATH } from '@/routing/resourceLinks';

export function Docker() {
  const location = useLocation();
  const pathname = () => location.pathname.replace(/\/+$/, '');

  if (pathname() === `${DOCKER_PATH}/workloads`) {
    return <Navigate href={buildDockerPath()} />;
  }

  return <DockerPageSurface />;
}

export default Docker;
