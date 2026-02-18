import type { Component } from 'solid-js';
import { createEffect, createMemo } from 'solid-js';
import { Show } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';

import Recovery from '@/components/Recovery/Recovery';
import { getRecoveryLegacyQueryRedirectTarget } from '@/routing/recoveryLegacyQueryRedirect';

const RecoveryRoute: Component = () => {
  const location = useLocation();
  const navigate = useNavigate();

  const redirectTarget = createMemo(() => getRecoveryLegacyQueryRedirectTarget(location.search));

  createEffect(() => {
    const target = redirectTarget();
    if (!target) return;
    const current = `${location.pathname}${location.search || ''}`;
    if (target === current) return;
    navigate(target, { replace: true });
  });

  return (
    <Show when={!redirectTarget()}>
      <Recovery />
    </Show>
  );
};

export default RecoveryRoute;

