package investigation

import "errors"

// ErrVerificationUnknown indicates the verifier could not conclusively determine
// whether a fix resolved the underlying issue. Callers may treat this as a
// distinct outcome from "verification failed" (issue persists).
var ErrVerificationUnknown = errors.New("verification inconclusive")
