import { onCleanup } from 'solid-js';

// A read belongs to one revision of an editor, not merely to equal final
// inputs: A → B → A must still retire it. Keep credentials out of signatures
// and logs. Invalidating evidence does not cancel the request or release its
// busy flag; its owner must remain busy until the request actually settles.
export const createConnectionEvidenceScope = () => {
  let revision = 0;
  let disposed = false;
  const invalidate = () => {
    revision++;
  };
  onCleanup(() => {
    disposed = true;
    invalidate();
  });
  return {
    invalidate,
    capture: () => {
      const captured = revision;
      return () => !disposed && captured === revision;
    },
  };
};
