import { Component, For } from 'solid-js';
import { Portal } from 'solid-js/web';
import NotificationToast from './NotificationToast';
import { notificationStore } from '../stores/notifications';

const NotificationContainer: Component = () => {
  return (
    <Portal>
      <div class="fixed top-4 right-4 z-50 space-y-2">
        <For each={notificationStore.notifications}>
          {(notification) => (
            <NotificationToast
              message={notification.message}
              type={notification.type}
              duration={notification.duration}
              onClose={() => notificationStore.remove(notification.id)}
            />
          )}
        </For>
      </div>
    </Portal>
  );
};

export default NotificationContainer;
