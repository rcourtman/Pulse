package agentexec

import (
	"errors"
	"testing"
)

// A caller that cannot ask for approval needs to tell this refusal apart from
// a transport failure, so it is a named error rather than a message.
func TestUnapprovedCommandReturnsTheNamedApprovalError(t *testing.T) {
	s := &Server{commandPolicy: DefaultPolicy()}
	for _, command := range []string{
		"du -xh -d 3 /mnt/nvr | sort -rh | head -20",
		"du -sh /mnt/nvr/frigate; ls -la /mnt/nvr/frigate",
		"df -h && echo ok",
	} {
		err := s.authorizeCommandPayload(ExecuteCommandPayload{Command: command})
		if !errors.Is(err, ErrCommandRequiresApproval) {
			t.Fatalf("%q: error = %v, want ErrCommandRequiresApproval", command, err)
		}
	}
	if err := s.authorizeCommandPayload(ExecuteCommandPayload{Command: "du -xh -d 3 /mnt/nvr"}); err != nil {
		t.Fatalf("a single auto-approved command was refused: %v", err)
	}
}
