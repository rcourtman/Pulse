// Fixed client/builder reason vocabulary, not a diagnosis of a native QGA fault.
export const guestDiskDeferrals = [
  [
    'vm-locked',
    'Guest reads paused while Proxmox reports a VM operation lock, such as a backup. Pulse will check again on a later poll.',
  ],
  [
    'lock-unverified',
    'Guest reads deferred because Pulse cannot verify that the VM is unlocked. Pulse will check again on a later poll.',
  ],
  [
    'agent-busy',
    'Guest reads deferred while an earlier guest request is still in progress. Pulse will check again on a later poll.',
  ],
  [
    'agent-cooldown',
    'Guest reads paused after an earlier request did not complete reliably. Pulse will check again after the cooldown.',
  ],
  [
    'agent-response-incomplete',
    'Guest reads paused because the previous response was incomplete. Pulse will check again on a later poll.',
  ],
  [
    'agent-completion-unverified',
    'Guest reads paused because completion of the earlier request could not be verified. Pulse will check again after the cooldown. Defer live diagnostics and guest-agent restarts during backups or a guest incident.',
  ],
  [
    'agent-redirect',
    'Guest reads paused because the endpoint returned a redirect. Pulse did not follow it; completion of the original guest request is uncertain. Defer live diagnostics and guest-agent restarts during backups or a guest incident.',
  ],
  [
    'agent-transport-unverified',
    'Guest reads deferred because Pulse cannot verify that the configured connection avoids resending guest requests. This guest request was not sent. Defer connection changes and live probes during backups or a guest incident.',
  ],
  [
    'agent-capacity',
    'Guest reads deferred because Pulse has reached its guest-read capacity. Pulse will check again on a later poll.',
  ],
  ['invalid-guest-key', 'Guest reads unavailable because the VM identity is invalid.'],
  [
    'agent-timeout',
    'Guest request timed out. Completion is uncertain. Do not restart the guest agent during a backup.',
  ],
] as const;
