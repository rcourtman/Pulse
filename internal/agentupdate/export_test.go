package agentupdate

import "context"

// PerformUpdateWithExecPathForTest runs the real update path while suppressing
// the process restart side effect so integration tests can inspect the result.
func PerformUpdateWithExecPathForTest(u *Updater, ctx context.Context, execPath string) error {
	origRestart := restartProcessFn
	restartProcessFn = func(string) error { return nil }
	defer func() {
		restartProcessFn = origRestart
	}()
	return u.performUpdateWithExecPath(ctx, execPath)
}

// WithExecutablePathForTest temporarily points update-info lookups at the
// supplied executable path so tests can exercise the real one-shot handoff.
func WithExecutablePathForTest(execPath string, fn func()) {
	origExec := osExecutableFn
	origEval := evalSymlinksFn
	osExecutableFn = func() (string, error) { return execPath, nil }
	evalSymlinksFn = func(string) (string, error) { return execPath, nil }
	defer func() {
		osExecutableFn = origExec
		evalSymlinksFn = origEval
	}()
	fn()
}
