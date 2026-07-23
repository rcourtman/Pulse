package agentupdate

import "context"

// PerformUpdateWithExecPathForTest runs the real update path while suppressing
// the process restart side effect so integration tests can inspect the result.
func PerformUpdateWithExecPathForTest(u *Updater, ctx context.Context, execPath string) error {
	origRestart := restartProcessFn
	origSelfTest := u.selfTestFn
	restartProcessFn = func(string) error { return nil }
	u.selfTestFn = func(context.Context, string) error { return nil }
	defer func() {
		restartProcessFn = origRestart
		u.selfTestFn = origSelfTest
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

// UseExecPathForUpdateChecksForTest wires CheckAndUpdate to the real update
// implementation while forcing a deterministic executable path.
func UseExecPathForUpdateChecksForTest(u *Updater, execPath string) func() {
	origPerform := u.performUpdateFn
	origExec := osExecutableFn
	origEval := evalSymlinksFn
	origRestart := restartProcessFn
	origSelfTest := u.selfTestFn

	osExecutableFn = func() (string, error) { return execPath, nil }
	evalSymlinksFn = func(string) (string, error) { return execPath, nil }
	restartProcessFn = func(string) error { return nil }
	u.selfTestFn = func(context.Context, string) error { return nil }
	u.performUpdateFn = func(ctx context.Context, targetVersion string) error {
		return u.performUpdateWithExecPathForVersion(ctx, execPath, targetVersion)
	}

	return func() {
		u.performUpdateFn = origPerform
		u.selfTestFn = origSelfTest
		osExecutableFn = origExec
		evalSymlinksFn = origEval
		restartProcessFn = origRestart
	}
}
