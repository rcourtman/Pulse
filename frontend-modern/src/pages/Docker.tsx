import { Navigate, useLocation } from '@solidjs/router';
import { Show } from 'solid-js';
import { DockerPageSurface } from '@/features/docker/DockerPageSurface';
import { buildDockerPath, DOCKER_PATH } from '@/routing/resourceLinks';

export function Docker() {
  const location = useLocation();
  const pathname = () => location.pathname.replace(/\/+$/, '');

  return (
    <Show
      when={pathname() !== `${DOCKER_PATH}/workloads`}
      fallback={<Navigate href={buildDockerPath()} />}
    >
      <DockerPageSurface />
    </Show>
  );
}

export default Docker;
