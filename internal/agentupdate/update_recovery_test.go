package agentupdate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

var (
	recoveryELFBinary = append([]byte{0x7f, 'E', 'L', 'F'}, bytes.Repeat([]byte("x"), 128)...)
	recoveryPEBinary  = append([]byte{'M', 'Z'}, bytes.Repeat([]byte("x"), 130)...)
)

// recoveryUpdateClient answers the version check and binary download in
// memory, so CheckAndUpdate runs end to end without a socket.
func recoveryUpdateClient(version string, binary []byte, downloadStatus int, downloadStatusText string) *http.Client {
	return &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		header := make(http.Header)
		status, statusText, body := http.StatusOK, "200 OK", []byte(nil)
		switch r.URL.Path {
		case "/api/agent/version":
			body = []byte(fmt.Sprintf(`{"version":%q}`, version))
		case "/download/pulse-agent":
			if downloadStatus != http.StatusOK {
				status, statusText = downloadStatus, downloadStatusText
				break
			}
			header.Set(checksumSHA256Header, checksum(binary))
			header.Set(signatureHeader, "signed-update")
			body = binary
		default:
			status, statusText = http.StatusNotFound, "404 Not Found"
		}
		return &http.Response{
			StatusCode:    status,
			Status:        statusText,
			Header:        header,
			Body:          io.NopCloser(bytes.NewReader(body)),
			ContentLength: int64(len(body)),
			Request:       r,
		}, nil
	})}
}

// stubRecoveryProcess fakes the process boundary: the executable path, the
// in-place restart (which returns restartErr) and the exit call.
func stubRecoveryProcess(t *testing.T, execPath string, restartErr error) *[]int {
	t.Helper()
	origExecutable, origEval, origRestart, origExit := osExecutableFn, evalSymlinksFn, restartProcessFn, exitFn
	t.Cleanup(func() {
		osExecutableFn, evalSymlinksFn, restartProcessFn, exitFn = origExecutable, origEval, origRestart, origExit
	})
	exits := &[]int{}
	osExecutableFn = func() (string, error) { return execPath, nil }
	evalSymlinksFn = func(path string) (string, error) { return path, nil }
	restartProcessFn = func(string) error { return restartErr }
	exitFn = func(code int) { *exits = append(*exits, code) }
	return exits
}

// isolateRecoveryPlatformProbes keeps a host's own Unraid or QNAP marker files
// from adding persistent-copy steps to a recorded replacement.
func isolateRecoveryPlatformProbes(t *testing.T) {
	t.Helper()
	missing := filepath.Join(t.TempDir(), "absent")
	origUnraid, origGetcfg, origQpkg := unraidVersionPath, qnapGetcfgPath, qnapQpkgConfPath
	t.Cleanup(func() {
		unraidVersionPath, qnapGetcfgPath, qnapQpkgConfPath = origUnraid, origGetcfg, origQpkg
	})
	unraidVersionPath, qnapGetcfgPath, qnapQpkgConfPath = missing, missing, missing
}

// useRecoveryPlatform selects which replacement path runs, whatever the host,
// and returns a payload whose magic that platform accepts.
func useRecoveryPlatform(t *testing.T, goos goOS) []byte {
	t.Helper()
	original := runtimeGOOS
	t.Cleanup(func() { runtimeGOOS = original })
	runtimeGOOS = goos
	if goos == goOSWindows {
		return recoveryPEBinary
	}
	return recoveryELFBinary
}

// newHelperRecoveryUpdater wires CheckAndUpdate to the fake typed helper with
// an in-place restart that fails after activation, as restartProcess reports it.
func newHelperRecoveryUpdater(t *testing.T, rollbackErr error) (*fakePrivilegedUpdate, *Updater, *[]int) {
	t.Helper()
	binary := useRecoveryPlatform(t, goOSLinux)
	helper := &fakePrivilegedUpdate{root: t.TempDir(), rollbackErr: rollbackErr}
	if err := securityutil.HardenPrivatePath(helper.root, 0o700); err != nil {
		t.Fatal(err)
	}
	exits := stubRecoveryProcess(t, "/usr/local/bin/pulse-agent", fmt.Errorf("failed to restart: %w", errors.New("exec refused")))
	u := New(Config{
		PulseURL:         "https://pulse.example.com",
		AgentName:        "pulse-agent",
		CurrentVersion:   "1.0.0",
		StateDir:         helper.root,
		PrivilegedUpdate: helper,
	})
	u.client = recoveryUpdateClient("1.1.0", binary, http.StatusOK, "")
	u.selfTestFn = func(context.Context, string) error { return nil }
	return helper, u, exits
}

var helperRestartRollbackEvents = []string{"quarantine", "signature:signed-update", "stage", "activate", "cleanup", "rollback"}

// A helper-backed update whose in-place restart fails and is rolled back has
// restored the running binary. Base matched "failed to restart" in that error
// and called os.Exit(1), so the failure was never reported and the restarted
// agent began with an idle, error-free status.
func TestCheckAndUpdateReportsRolledBackHelperRestartFailureWithoutExiting(t *testing.T) {
	helper, u, exits := newHelperRecoveryUpdater(t, nil)

	u.CheckAndUpdate(context.Background())

	if len(*exits) != 0 {
		t.Fatalf("exit codes = %v, want none after the helper restored the running binary", *exits)
	}
	status := u.Snapshot()
	if status.State != UpdateStateError || status.LastSuccessAt != nil || !strings.Contains(status.LastError, "update rolled back") {
		t.Fatalf("status after rolled-back helper update = %+v", status)
	}
	if !reflect.DeepEqual(helper.events, helperRestartRollbackEvents) {
		t.Fatalf("helper events = %#v, want %#v", helper.events, helperRestartRollbackEvents)
	}
	if pending, err := LoadPendingPrivilegedUpdate(helper.root); err != nil || pending != nil {
		t.Fatalf("pending handoff after rollback = %#v, %v", pending, err)
	}
}

// A binary restored on disk is not a settled transaction. When clearing the
// local handoff fails after the helper rolled back, nothing on disk differs
// from the running binary, so the process keeps running and reports both
// facts; the stale handoff stays visible instead of being treated as settled.
func TestCheckAndUpdateReportsStaleHandoffWhenClearFailsAfterRollback(t *testing.T) {
	helper, u, exits := newHelperRecoveryUpdater(t, nil)
	origClear := clearPendingUpdateFn
	t.Cleanup(func() { clearPendingUpdateFn = origClear })
	clearPendingUpdateFn = func(string) error { return errors.New("fixture clear refused") }

	u.CheckAndUpdate(context.Background())

	if len(*exits) != 0 {
		t.Fatalf("exit codes = %v, want none: the binary on disk is the running one again", *exits)
	}
	status := u.Snapshot()
	if status.State != UpdateStateError || !strings.Contains(status.LastError, "update rolled back") ||
		!strings.Contains(status.LastError, "clear pending update handoff: fixture clear refused") {
		t.Fatalf("status after rollback with a failed clear = %+v", status)
	}
	if !reflect.DeepEqual(helper.events, helperRestartRollbackEvents) {
		t.Fatalf("helper events = %#v, want %#v", helper.events, helperRestartRollbackEvents)
	}
	pending, err := LoadPendingPrivilegedUpdate(helper.root)
	if err != nil || pending == nil || pending.Activation.ActivationID != helper.activation.ActivationID {
		t.Fatalf("stale handoff after failed clear = %#v, %v; want the rolled-back activation still recorded", pending, err)
	}
}

// When the helper cannot roll back, the activated binary stays on disk under a
// persisted pending handoff while this process runs the old one, so the exit
// that hands over to the service manager is kept.
func TestCheckAndUpdateExitsWhenHelperCannotRollBackRestartFailure(t *testing.T) {
	helper, u, exits := newHelperRecoveryUpdater(t, errors.New("helper unavailable"))

	u.CheckAndUpdate(context.Background())

	if !reflect.DeepEqual(*exits, []int{1}) {
		t.Fatalf("exit codes = %v, want [1]", *exits)
	}
	if status := u.Snapshot(); status.State != UpdateStateError || !strings.Contains(status.LastError, "typed helper rollback failed") {
		t.Fatalf("status after failed rollback = %+v", status)
	}
	pending, err := LoadPendingPrivilegedUpdate(helper.root)
	if err != nil || pending == nil || pending.Activation.ActivationID != helper.activation.ActivationID {
		t.Fatalf("pending handoff after failed rollback = %#v, %v", pending, err)
	}
}

// A download refused before any replacement leaves the running binary in
// place. Base exited anyway when the server's status text contained "failed
// to restart", dropping the error it had just recorded.
func TestCheckAndUpdateKeepsRunningWhenPreReplacementErrorMentionsRestart(t *testing.T) {
	isolateRecoveryPlatformProbes(t)
	binary := useRecoveryPlatform(t, goOSLinux)
	_, execPath := writeTempExec(t)
	exits := stubRecoveryProcess(t, execPath, nil)
	u := newUpdaterForTest("https://pulse.example.com")
	u.client = recoveryUpdateClient("1.1.0", binary, http.StatusNotFound, "404 failed to restart")

	u.CheckAndUpdate(context.Background())

	if len(*exits) != 0 {
		t.Fatalf("exit codes = %v, want none before any replacement", *exits)
	}
	if status := u.Snapshot(); status.State != UpdateStateError || !strings.Contains(status.LastError, "404 failed to restart") {
		t.Fatalf("status after refused download = %+v", status)
	}
	if got, err := os.ReadFile(execPath); err != nil || string(got) != "old-binary" {
		t.Fatalf("executable after refused download = %q, %v", got, err)
	}
}

type fsStep struct {
	op          string // create, close, chmod, syncfile, rename, remove, writefile, syncdir or restart
	path, to    string
	mode        os.FileMode
	servicePath bool // whether the service path existed after the step
}

type fsRecorder struct {
	servicePath string
	steps       []fsStep
	syncFileErr error
	syncDirErr  error
	restartErr  error
	failRename  func(from, to string) bool
}

func (r *fsRecorder) record(op, path, to string, mode os.FileMode) {
	_, err := os.Lstat(r.servicePath)
	r.steps = append(r.steps, fsStep{op: op, path: path, to: to, mode: mode, servicePath: err == nil})
}

func (r *fsRecorder) ops() []string {
	ops := make([]string, 0, len(r.steps))
	for _, step := range r.steps {
		ops = append(ops, step.op)
	}
	return ops
}

// recordUpdateFS routes the replacement's filesystem seams through a recorder.
// Creates, closes, chmods, renames, removals and writes really happen inside
// t.TempDir(); the durability barriers and the restart only record, or return
// the configured fixture error.
func recordUpdateFS(t *testing.T, servicePath string) *fsRecorder {
	t.Helper()
	rec := &fsRecorder{servicePath: servicePath}
	origCreate, origClose, origChmod, origRename := createTempFn, closeFileFn, chmodFn, renameFn
	origRemove, origWrite, origSyncFile, origSyncDir, origRestart := removeFileFn, writeFileFn, syncFileFn, syncDirFn, restartProcessFn
	t.Cleanup(func() {
		createTempFn, closeFileFn, chmodFn, renameFn = origCreate, origClose, origChmod, origRename
		removeFileFn, writeFileFn, syncFileFn, syncDirFn, restartProcessFn = origRemove, origWrite, origSyncFile, origSyncDir, origRestart
	})
	createTempFn = func(dir, pattern string) (*os.File, error) {
		file, err := origCreate(dir, pattern)
		if err == nil {
			rec.record("create", file.Name(), "", 0)
		}
		return file, err
	}
	closeFileFn = func(file *os.File) error {
		err := origClose(file)
		if err == nil {
			rec.record("close", file.Name(), "", 0)
		}
		return err
	}
	chmodFn = func(path string, mode os.FileMode) error {
		err := origChmod(path, mode)
		if err == nil {
			rec.record("chmod", path, "", mode)
		}
		return err
	}
	renameFn = func(from, to string) error {
		if rec.failRename != nil && rec.failRename(from, to) {
			return errors.New("fixture rename refused")
		}
		err := origRename(from, to)
		if err == nil {
			rec.record("rename", from, to, 0)
		}
		return err
	}
	removeFileFn = func(path string) error {
		err := origRemove(path)
		if err == nil {
			rec.record("remove", path, "", 0)
		}
		return err
	}
	writeFileFn = func(path string, data []byte, mode os.FileMode) error {
		err := origWrite(path, data, mode)
		if err == nil {
			rec.record("writefile", path, "", mode)
		}
		return err
	}
	syncFileFn = func(path string) error {
		if rec.syncFileErr != nil {
			return rec.syncFileErr
		}
		rec.record("syncfile", path, "", 0)
		return nil
	}
	syncDirFn = func(dir string) error {
		if rec.syncDirErr != nil {
			return rec.syncDirErr
		}
		rec.record("syncdir", dir, "", 0)
		return nil
	}
	restartProcessFn = func(string) error {
		rec.record("restart", "", "", 0)
		return rec.restartErr
	}
	return rec
}

type modelInode struct {
	complete, durableComplete bool
	mode, durableMode         os.FileMode
	updateInfo                bool
}

func describeServiceInode(inode *modelInode, durable bool) string {
	if inode == nil {
		return "missing"
	}
	complete, mode := inode.complete, inode.mode
	if durable {
		complete, mode = inode.durableComplete, inode.durableMode
	}
	if !complete {
		return "naming incomplete bytes"
	}
	if mode&0o111 == 0 {
		return fmt.Sprintf("not executable (%v)", mode)
	}
	return "valid"
}

// firstPowerLossViolation replays recorded steps against a deliberately
// pessimistic crash model. It is an assumption, not a measured filesystem: a
// file's bytes and mode reach disk only through a sync of that file after they
// were set, and each rename, removal or new file reaches disk independently,
// in any combination, unless a later sync of its directory forced it. After
// every step the visible service path, and in every combination the service
// path on disk, must name a complete executable; update evidence on disk must
// never outlive the replacement it describes. It returns the first violation.
func firstPowerLossViolation(steps []fsStep, servicePath string) string {
	type nsOp struct {
		step   fsStep
		inode  *modelInode
		forced bool
	}
	for crash := 0; crash <= len(steps); crash++ {
		previous := &modelInode{complete: true, durableComplete: true, mode: 0o755, durableMode: 0o755}
		visible := map[string]*modelInode{servicePath: previous}
		var ops []nsOp
		for _, step := range steps[:crash] {
			switch step.op {
			case "create":
				visible[step.path] = &modelInode{mode: 0o600}
				ops = append(ops, nsOp{step: step, inode: visible[step.path], forced: true})
			case "writefile":
				visible[step.path] = &modelInode{complete: true, mode: step.mode, updateInfo: true}
				ops = append(ops, nsOp{step: step, inode: visible[step.path]})
			case "close":
				if inode := visible[step.path]; inode != nil {
					inode.complete = true
				}
			case "chmod":
				if inode := visible[step.path]; inode != nil {
					inode.mode = step.mode
				}
			case "syncfile":
				if inode := visible[step.path]; inode != nil {
					inode.durableComplete, inode.durableMode = inode.complete, inode.mode
				}
			case "rename":
				visible[step.to] = visible[step.path]
				delete(visible, step.path)
				ops = append(ops, nsOp{step: step})
			case "remove":
				delete(visible, step.path)
				ops = append(ops, nsOp{step: step})
			case "syncdir":
				for i := range ops {
					if filepath.Dir(ops[i].step.path) == step.path {
						ops[i].forced = true
					}
				}
			}
		}
		if state := describeServiceInode(visible[servicePath], false); state != "valid" {
			return fmt.Sprintf("after step %d the visible service path is %s", crash, state)
		}
		var optional []int
		for i, op := range ops {
			if !op.forced {
				optional = append(optional, i)
			}
		}
		for mask := 0; mask < 1<<len(optional); mask++ {
			onDisk := make([]bool, len(ops))
			for i, op := range ops {
				onDisk[i] = op.forced
			}
			for bit, i := range optional {
				onDisk[i] = mask&(1<<bit) != 0
			}
			durable := map[string]*modelInode{servicePath: previous}
			var applied []string
			for i, op := range ops {
				if !onDisk[i] {
					continue
				}
				switch op.step.op {
				case "create", "writefile":
					durable[op.step.path] = op.inode
				case "rename":
					if inode, ok := durable[op.step.path]; ok {
						durable[op.step.to] = inode
						delete(durable, op.step.path)
					}
				case "remove":
					delete(durable, op.step.path)
				}
				if op.step.op != "create" {
					applied = append(applied, op.step.op+" "+filepath.Base(op.step.path))
				}
			}
			service := durable[servicePath]
			if state := describeServiceInode(service, true); state != "valid" {
				return fmt.Sprintf("power loss after step %d with %v on disk leaves the service path %s", crash, applied, state)
			}
			for _, inode := range durable {
				if inode.updateInfo && service == previous {
					return fmt.Sprintf("power loss after step %d with %v on disk keeps update evidence for a lost replacement", crash, applied)
				}
			}
		}
	}
	return ""
}

// Off Windows the staged binary is renamed over the service path only after
// its bytes and final executable mode were synced, the directory is synced
// before any cleanup or update evidence, and so after any step, or a power
// loss in the model above, the service path names a complete executable.
// Base moved the running binary aside first, leaving the path empty, and
// synced nothing.
func TestLocalReplacementKeepsExecutableServicePathThroughPowerLoss(t *testing.T) {
	isolateRecoveryPlatformProbes(t)
	binary := useRecoveryPlatform(t, goOSLinux)
	_, execPath := writeTempExec(t)
	stubRecoveryProcess(t, execPath, nil)
	rec := recordUpdateFS(t, execPath)
	u := newUpdaterForTest("https://pulse.example.com")
	u.client = recoveryUpdateClient("1.1.0", binary, http.StatusOK, "")

	if err := u.performUpdateWithExecPath(context.Background(), execPath); err != nil {
		t.Fatalf("performUpdateWithExecPath: %v", err)
	}

	if violation := firstPowerLossViolation(rec.steps, execPath); violation != "" {
		t.Fatalf("%s; steps=%+v", violation, rec.steps)
	}
	for i, step := range rec.steps {
		if !step.servicePath {
			t.Fatalf("service path missing after step %d: %+v", i, step)
		}
	}
	want := []string{"create", "close", "chmod", "syncfile", "rename", "syncdir", "writefile", "restart"}
	if got := rec.ops(); !reflect.DeepEqual(got, want) {
		t.Fatalf("replacement steps = %v, want %v", got, want)
	}
	if got, err := os.ReadFile(execPath); err != nil || !bytes.Equal(got, binary) {
		t.Fatalf("executable after update = %q, %v", got, err)
	}
}

// A staged binary that cannot be made durable is a pre-replacement failure:
// the running binary stays untouched and nothing is renamed or restarted.
func TestLocalReplacementStagedSyncFailureKeepsRunningBinary(t *testing.T) {
	isolateRecoveryPlatformProbes(t)
	binary := useRecoveryPlatform(t, goOSLinux)
	dir, execPath := writeTempExec(t)
	stubRecoveryProcess(t, execPath, nil)
	rec := recordUpdateFS(t, execPath)
	rec.syncFileErr = errors.New("fixture fsync refused")
	u := newUpdaterForTest("https://pulse.example.com")
	u.client = recoveryUpdateClient("1.1.0", binary, http.StatusOK, "")

	err := u.performUpdateWithExecPath(context.Background(), execPath)
	if err == nil || !strings.Contains(err.Error(), "failed to sync staged binary") {
		t.Fatalf("performUpdateWithExecPath error = %v", err)
	}
	if got := rec.ops(); !reflect.DeepEqual(got, []string{"create", "close", "chmod"}) {
		t.Fatalf("steps after sync failure = %v, want create, close, chmod only", got)
	}
	if got, readErr := os.ReadFile(execPath); readErr != nil || string(got) != "old-binary" {
		t.Fatalf("executable after sync failure = %q, %v", got, readErr)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, "pulse-agent-*.tmp")); len(leftovers) != 0 {
		t.Fatalf("staged temp files left behind: %v", leftovers)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".pulse-update-info")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("update info written after a pre-replacement failure: %v", statErr)
	}
}

// Off Windows a refused rename leaves the service path exactly as it was: it
// is never moved aside, so there is nothing to restore. Base moved it aside
// first and depended on a second rename to put it back.
func TestLocalReplacementRenameFailureNeverEmptiesServicePath(t *testing.T) {
	isolateRecoveryPlatformProbes(t)
	binary := useRecoveryPlatform(t, goOSLinux)
	dir, execPath := writeTempExec(t)
	stubRecoveryProcess(t, execPath, nil)
	rec := recordUpdateFS(t, execPath)
	rec.failRename = func(from, to string) bool { return to == execPath && strings.HasSuffix(from, ".tmp") }
	u := newUpdaterForTest("https://pulse.example.com")
	u.client = recoveryUpdateClient("1.1.0", binary, http.StatusOK, "")

	err := u.performUpdateWithExecPath(context.Background(), execPath)
	if err == nil || !strings.Contains(err.Error(), "failed to replace binary") {
		t.Fatalf("performUpdateWithExecPath error = %v", err)
	}
	for i, step := range rec.steps {
		if !step.servicePath || (step.op == "rename" && step.path == execPath) {
			t.Fatalf("service path moved or missing at step %d: %+v", i, step)
		}
	}
	if got := rec.ops(); !reflect.DeepEqual(got, []string{"create", "close", "chmod", "syncfile"}) {
		t.Fatalf("steps after refused rename = %v", got)
	}
	if got, readErr := os.ReadFile(execPath); readErr != nil || string(got) != "old-binary" {
		t.Fatalf("executable after refused rename = %q, %v", got, readErr)
	}
	if _, statErr := os.Stat(execPath + ".backup"); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("backup created although the service path was never moved: %v", statErr)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, "pulse-agent-*.tmp")); len(leftovers) != 0 {
		t.Fatalf("staged temp files left behind: %v", leftovers)
	}
}

// A failed directory sync leaves the replacement without a durability
// guarantee, so cleanup waits and any backup stays. The visible replacement is
// complete and the restart follows, so the "updated from" record is still
// written, which also replaces an earlier record nobody consumed. r2 skipped
// the write and left that earlier record to misattribute this update. No
// crash-durability claim is made on this path.
func TestLocalReplacementDirectorySyncFailureKeepsBackupAndReplacesStaleEvidence(t *testing.T) {
	isolateRecoveryPlatformProbes(t)
	binary := useRecoveryPlatform(t, goOSLinux)
	dir, execPath := writeTempExec(t)
	if err := os.WriteFile(execPath+".backup", []byte("stale-backup"), 0o755); err != nil {
		t.Fatal(err)
	}
	infoPath := filepath.Join(dir, ".pulse-update-info")
	if err := os.WriteFile(infoPath, []byte("0.9.0"), 0o644); err != nil {
		t.Fatal(err)
	}
	stubRecoveryProcess(t, execPath, nil)
	rec := recordUpdateFS(t, execPath)
	rec.syncDirErr = errors.New("fixture directory fsync refused")
	u := newUpdaterForTest("https://pulse.example.com")
	u.client = recoveryUpdateClient("1.1.0", binary, http.StatusOK, "")

	if err := u.performUpdateWithExecPath(context.Background(), execPath); err != nil {
		t.Fatalf("performUpdateWithExecPath: %v", err)
	}
	if got := rec.ops(); !reflect.DeepEqual(got, []string{"create", "close", "chmod", "syncfile", "rename", "writefile", "restart"}) {
		t.Fatalf("steps with an unsynced directory = %v", got)
	}
	if got, err := os.ReadFile(execPath); err != nil || !bytes.Equal(got, binary) {
		t.Fatalf("executable after update = %q, %v", got, err)
	}
	if got, err := os.ReadFile(execPath + ".backup"); err != nil || string(got) != "stale-backup" {
		t.Fatalf("backup after unsynced directory = %q, %v", got, err)
	}
	if got, err := os.ReadFile(infoPath); err != nil || string(got) != "1.0.0" {
		t.Fatalf("update info = %q, %v; want this update's previous version, not the stale record", got, err)
	}
}

// Windows cannot rename over a running image, so its path still moves the
// running binary aside and the service path is empty until the second rename.
// This pins that platform gap so it is not mistaken for solved; the barriers
// around it are the same as elsewhere.
func TestWindowsReplacementPinsMissingServicePathGap(t *testing.T) {
	isolateRecoveryPlatformProbes(t)
	binary := useRecoveryPlatform(t, goOSWindows)
	_, execPath := writeTempExec(t)
	stubRecoveryProcess(t, execPath, nil)
	rec := recordUpdateFS(t, execPath)
	u := newUpdaterForTest("https://pulse.example.com")
	u.client = recoveryUpdateClient("1.1.0", binary, http.StatusOK, "")

	if err := u.performUpdateWithExecPath(context.Background(), execPath); err != nil {
		t.Fatalf("performUpdateWithExecPath: %v", err)
	}
	want := []string{"create", "close", "chmod", "syncfile", "rename", "rename", "syncdir", "remove", "writefile", "restart"}
	if got := rec.ops(); !reflect.DeepEqual(got, want) {
		t.Fatalf("windows replacement steps = %v, want %v", got, want)
	}
	if aside := rec.steps[4]; aside.path != execPath || aside.servicePath {
		t.Fatalf("step 4 = %+v, want the running binary moved aside leaving the service path empty", aside)
	}
	if violation := firstPowerLossViolation(rec.steps, execPath); !strings.Contains(violation, "visible service path is missing") {
		t.Fatalf("power-loss model = %q, want the documented missing-path gap", violation)
	}
	if got, err := os.ReadFile(execPath); err != nil || !bytes.Equal(got, binary) {
		t.Fatalf("executable after update = %q, %v", got, err)
	}
}

// The Windows path keeps restoring the moved-aside binary when the second
// rename fails, as base did.
func TestWindowsReplacementRestoresBackupWhenSecondRenameFails(t *testing.T) {
	isolateRecoveryPlatformProbes(t)
	binary := useRecoveryPlatform(t, goOSWindows)
	_, execPath := writeTempExec(t)
	stubRecoveryProcess(t, execPath, nil)
	rec := recordUpdateFS(t, execPath)
	rec.failRename = func(from, to string) bool { return to == execPath && strings.HasSuffix(from, ".tmp") }
	u := newUpdaterForTest("https://pulse.example.com")
	u.client = recoveryUpdateClient("1.1.0", binary, http.StatusOK, "")

	err := u.performUpdateWithExecPath(context.Background(), execPath)
	if err == nil || !strings.Contains(err.Error(), "failed to replace binary") {
		t.Fatalf("performUpdateWithExecPath error = %v", err)
	}
	var renames []string
	for _, step := range rec.steps {
		switch step.op {
		case "rename":
			renames = append(renames, filepath.Base(step.path)+"->"+filepath.Base(step.to))
		case "restart":
			t.Fatalf("restart after a failed replacement: %+v", rec.steps)
		}
	}
	base := filepath.Base(execPath)
	if want := []string{base + "->" + base + ".backup", base + ".backup->" + base}; !reflect.DeepEqual(renames, want) {
		t.Fatalf("renames = %v, want %v", renames, want)
	}
	if got, readErr := os.ReadFile(execPath); readErr != nil || string(got) != "old-binary" {
		t.Fatalf("executable after restore = %q, %v", got, readErr)
	}
	if _, statErr := os.Stat(execPath + ".backup"); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("backup left after restore: %v", statErr)
	}
}

// The local path replaces the executable before restarting, so a failed exec
// still exits for the service manager to start the new binary.
func TestCheckAndUpdateExitsWhenLocalReplacementCannotRestart(t *testing.T) {
	isolateRecoveryPlatformProbes(t)
	binary := useRecoveryPlatform(t, goOSLinux)
	_, execPath := writeTempExec(t)
	exits := stubRecoveryProcess(t, execPath, nil)
	rec := recordUpdateFS(t, execPath)
	rec.restartErr = fmt.Errorf("failed to restart: %w", errors.New("exec format error"))
	u := newUpdaterForTest("https://pulse.example.com")
	u.client = recoveryUpdateClient("1.1.0", binary, http.StatusOK, "")

	u.CheckAndUpdate(context.Background())

	if !reflect.DeepEqual(*exits, []int{1}) {
		t.Fatalf("exit codes = %v, want [1]", *exits)
	}
	if ops := rec.ops(); len(ops) == 0 || ops[len(ops)-1] != "restart" {
		t.Fatalf("steps = %v, want the replacement to end in the restart attempt", ops)
	}
	if got, err := os.ReadFile(execPath); err != nil || !bytes.Equal(got, binary) {
		t.Fatalf("executable after local replacement = %q, %v", got, err)
	}
}

// The Unraid flash and QNAP persistent copies are what a reboot restores, so
// their bytes and final mode are synced before the rename and their directory
// after it; a copy that cannot be synced never replaces the previous one.
func TestPersistentBinaryCopySyncsFinalModeAroundRename(t *testing.T) {
	setup := func(t *testing.T) (string, string, *fsRecorder) {
		t.Helper()
		dir := t.TempDir()
		execPath := filepath.Join(dir, "running-agent")
		persistDir := filepath.Join(dir, "flash")
		if err := os.Mkdir(persistDir, 0o755); err != nil {
			t.Fatal(err)
		}
		persistPath := filepath.Join(persistDir, "pulse-agent")
		if err := os.WriteFile(execPath, []byte("new-binary"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(persistPath, []byte("old-binary"), 0o755); err != nil {
			t.Fatal(err)
		}
		return execPath, persistPath, recordUpdateFS(t, persistPath)
	}

	t.Run("SyncedAroundRename", func(t *testing.T) {
		execPath, persistPath, rec := setup(t)
		New(Config{}).syncPersistentBinaryCopy(execPath, persistPath, "unraid")
		want := []string{"create", "close", "chmod", "syncfile", "rename", "syncdir"}
		if got := rec.ops(); !reflect.DeepEqual(got, want) {
			t.Fatalf("persistent copy steps = %v, want %v", got, want)
		}
		if rec.steps[2].path != rec.steps[3].path || rec.steps[2].mode != 0o755 {
			t.Fatalf("sync %+v does not follow the final chmod %+v of the same file", rec.steps[3], rec.steps[2])
		}
		if violation := firstPowerLossViolation(rec.steps, persistPath); violation != "" {
			t.Fatalf("%s; steps=%+v", violation, rec.steps)
		}
		if got, err := os.ReadFile(persistPath); err != nil || string(got) != "new-binary" {
			t.Fatalf("persistent copy = %q, %v", got, err)
		}
	})

	t.Run("SyncFailureKeepsPreviousCopy", func(t *testing.T) {
		execPath, persistPath, rec := setup(t)
		rec.syncFileErr = errors.New("fixture fsync refused")
		New(Config{}).syncPersistentBinaryCopy(execPath, persistPath, "unraid")
		if got := rec.ops(); !reflect.DeepEqual(got, []string{"create", "close", "chmod"}) {
			t.Fatalf("persistent copy steps after sync failure = %v", got)
		}
		if got, err := os.ReadFile(persistPath); err != nil || string(got) != "old-binary" {
			t.Fatalf("persistent copy after sync failure = %q, %v", got, err)
		}
		if leftovers, _ := filepath.Glob(persistPath + "-persist-*.tmp"); len(leftovers) != 0 {
			t.Fatalf("persistent temp files left behind: %v", leftovers)
		}
	})
}
