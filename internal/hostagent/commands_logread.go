package hostagent

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"syscall"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

// logReadRootDir is the directory log reads are opened beneath. Tests point
// it at a scratch tree.
var logReadRootDir = agentexec.LogReadRoot

// maxLogReadFiles bounds the descriptors one log read holds open.
const maxLogReadFiles = 256

// maxLogReadLinks bounds the absolute links followed for one operand.
const maxLogReadLinks = 8

// confinedLogRead is a log read whose files the agent has opened itself.
//
// The command policy approves cat, head, tail and grep by path name, and a
// name under /var/log says nothing about the file behind it: the account that
// owns /var/log/<service> can replace its log with a symbolic link to a file
// only root may read. Checking where a name leads and then letting the
// program open it again leaves the owner time to swap the link in between. So
// every operand is opened here, beneath /var/log, with no link allowed to
// leave it, and the program is given the open descriptors (/dev/fd/N).
type confinedLogRead struct {
	// command is the program and its options, with a descriptor path in
	// place of every file operand.
	command string
	// files become descriptors 3, 4, ... of the program.
	files []*os.File
	// names turns descriptor paths in the output back into the names asked for.
	names *strings.Replacer
	// refused has one line for each operand that was not opened, worded the
	// way the program reports a file it cannot read.
	refused string
}

// confineLogRead opens the file operands of an approved log read. An operand
// that does not open beneath /var/log as a regular file or directory is
// left out and reported; it never reaches the program by name. A plain sudo
// prefix is not kept: sudo closes inherited descriptors, and the files are
// opened with the agent's own rights.
func confineLogRead(read agentexec.LogRead) (*confinedLogRead, error) {
	root, rootErr := os.OpenRoot(logReadRootDir)
	if rootErr == nil {
		defer root.Close()
	}

	confined := &confinedLogRead{}
	words := []string{read.Program}
	var names []string
	var refused strings.Builder
	for _, arg := range read.Args {
		if !arg.File {
			words = append(words, shellQuote(arg.Text))
			continue
		}
		for _, name := range expandLogReadOperand(root, arg) {
			file, err := openLogReadFile(root, rootErr, name)
			if err != nil {
				fmt.Fprintf(&refused, "%s: %s: %v\n", read.Program, name, err)
				continue
			}
			confined.files = append(confined.files, file)
			if len(confined.files) > maxLogReadFiles {
				confined.Close()
				return nil, fmt.Errorf("log read names more than %d files. Name fewer, or search the directory with grep -r", maxLogReadFiles)
			}
			if len(confined.files) == 1 && !descriptorPathReaches(file) {
				confined.Close()
				return nil, errors.New("log reads need /dev/fd on this host")
			}
			descriptor := fmt.Sprintf("/dev/fd/%d", 2+len(confined.files))
			words = append(words, descriptor)
			// Higher numbers first, so /dev/fd/30 is not read as /dev/fd/3.
			names = append([]string{descriptor, strings.TrimRight(name, "/")}, names...)
		}
	}
	confined.command = strings.Join(words, " ")
	confined.names = strings.NewReplacer(names...)
	confined.refused = refused.String()
	return confined, nil
}

// expandLogReadOperand returns the names a file operand stands for. The
// shell is not given the operand, so a glob is expanded here, inside the
// root, the way sh does it: sorted, a leading dot matched only by a dot, and
// the pattern itself when nothing matches.
func expandLogReadOperand(root *os.Root, arg agentexec.LogReadArg) []string {
	if !arg.Glob || root == nil {
		return []string{arg.Text}
	}
	pattern := path.Clean(logReadRelativeName(arg.Text))
	matches, err := fs.Glob(root.FS(), pattern)
	if err != nil {
		return []string{arg.Text}
	}
	segments := strings.Split(pattern, "/")
	var names []string
	for _, match := range matches {
		hidden := false
		for i, segment := range strings.Split(match, "/") {
			if strings.HasPrefix(segment, ".") && (i >= len(segments) || !strings.HasPrefix(segments[i], ".")) {
				hidden = true
			}
		}
		if !hidden {
			names = append(names, agentexec.LogReadRoot+"/"+match)
		}
	}
	if len(names) == 0 {
		return []string{arg.Text}
	}
	return names
}

// logReadRelativeName returns a name under /var/log relative to it.
func logReadRelativeName(name string) string {
	relative := strings.TrimLeft(strings.TrimPrefix(name, agentexec.LogReadRoot), "/")
	if relative == "" {
		return "."
	}
	return relative
}

// openLogReadFile opens one operand beneath the root. os.Root follows a
// symbolic link only while it stays inside the root, so a link to an absolute
// path or up past /var/log fails instead of opening its target. A FIFO,
// socket or device is refused: a log is a regular file.
func openLogReadFile(root *os.Root, rootErr error, name string) (*os.File, error) {
	var file *os.File
	err := rootErr
	if err == nil {
		relative := logReadRelativeName(name)
		// O_NONBLOCK: opening a FIFO must not wait here for a writer.
		file, err = root.OpenFile(relative, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		for links := 0; err != nil && links < maxLogReadLinks; links++ {
			next, ok := logReadLinkTarget(root, relative)
			if !ok {
				break
			}
			relative = next
			file, err = root.OpenFile(relative, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		}
	}
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			err = pathErr.Err
		}
		return nil, err
	}
	info, err := file.Stat()
	if err == nil && !info.Mode().IsRegular() && !info.IsDir() {
		err = errors.New("not a regular file or directory")
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

// logReadLinkTarget spells out the first absolute link in name that points
// back under /var/log, and returns the name that results, relative to the
// root. os.Root refuses every absolute link, and that is how kubelet links
// /var/log/containers/*.log to /var/log/pods. The result is only another
// name: the caller opens it through the root again, so it cannot lead outside
// /var/log either, whatever the link is changed to in the meantime.
func logReadLinkTarget(root *os.Root, name string) (string, bool) {
	segments := strings.Split(path.Clean(name), "/")
	for i := range segments {
		target, err := root.Readlink(path.Join(segments[:i+1]...))
		if err != nil || !path.IsAbs(target) {
			continue
		}
		target = path.Clean(target)
		if target != logReadRootDir && !strings.HasPrefix(target, logReadRootDir+"/") {
			return "", false
		}
		return path.Join(".", strings.TrimPrefix(target, logReadRootDir), path.Join(segments[i+1:]...)), true
	}
	return "", false
}

// descriptorPathReaches reports whether /dev/fd names this process's open
// descriptors. FreeBSD has only 0 to 2 there unless fdescfs is mounted.
func descriptorPathReaches(file *os.File) bool {
	want, err := file.Stat()
	if err != nil {
		return false
	}
	got, err := os.Stat(fmt.Sprintf("/dev/fd/%d", file.Fd()))
	return err == nil && os.SameFile(want, got)
}

// finish puts the names asked for back into the program's output and adds
// the operands that were refused. Like the programs themselves, a read that
// could not open every file does not report success.
func (r *confinedLogRead) finish(result *commandResultPayload) {
	result.Stdout = r.names.Replace(result.Stdout)
	result.Stderr = r.refused + r.names.Replace(result.Stderr)
	if r.refused != "" && result.Success {
		result.ExitCode = 1
		result.Success = false
	}
}

// Close releases the agent's copies of the descriptors.
func (r *confinedLogRead) Close() {
	for _, file := range r.files {
		file.Close()
	}
}
