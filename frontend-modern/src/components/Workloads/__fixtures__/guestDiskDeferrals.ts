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
    'agent-capacity',
    'Guest reads deferred because Pulse has reached its guest-read capacity. Pulse will check again on a later poll.',
  ],
  ['invalid-guest-key', 'Guest reads unavailable because the VM identity is invalid.'],
  [
    'agent-timeout',
    'Guest request timed out. Completion is uncertain. Do not restart the guest agent during a backup.',
  ],
] as const;
