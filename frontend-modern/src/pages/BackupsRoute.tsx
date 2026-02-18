import type { Component } from 'solid-js';
import { createEffect, createMemo } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { Show } from 'solid-js';

import Backups from '@/components/Backups/Backups';
import { getBackupsLegacyQueryRedirectTarget } from '@/routing/backupsLegacyQueryRedirect';

const BackupsRoute: Component = () => {
  const location = useLocation();
  const navigate = useNavigate();

  const redirectTarget = createMemo(() => getBackupsLegacyQueryRedirectTarget(location.search));

  createEffect(() => {
    const target = redirectTarget();
    if (!target) return;
    const current = `${location.pathname}${location.search || ''}`;
    if (target === current) return;
    navigate(target, { replace: true });
  });

  return (
    <Show when={!redirectTarget()}>
      <Backups />
    </Show>
  );
};

export default BackupsRoute;
