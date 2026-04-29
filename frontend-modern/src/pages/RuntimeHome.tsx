import { useNavigate } from '@solidjs/router';
import { createEffect } from 'solid-js';
import { buildInfrastructurePath } from '@/routing/resourceLinks';

export default function RuntimeHome() {
  const navigate = useNavigate();
  const destination = () => buildInfrastructurePath();

  createEffect(() => {
    navigate(destination(), { replace: true });
  });

  return <div class="px-4 py-6 text-sm text-muted">Opening workspace...</div>;
}
