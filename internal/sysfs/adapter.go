package sysfs

import (
	"fmt"
	"io/fs"
	"path"
	"syscall"

	"github.com/tetratelabs/wazero/internal/platform"
)

// Adapt adapts the input to FS unless it is already one. Use NewDirFS instead
// of os.DirFS as it handles interop issues such as windows support.
//
// Note: This performs no flag verification on FS.OpenFile. fs.FS cannot read
// flags as there is no parameter to pass them through with. Moreover, fs.FS
// documentation does not require the file to be present. In summary, we can't
// enforce flag behavior.
func Adapt(fs fs.FS) FS {
	if fs == nil {
		return UnimplementedFS{}
	}
	if sys, ok := fs.(FS); ok {
		return sys
	}
	return &adapter{fs: fs}
}

type adapter struct {
	UnimplementedFS
	fs fs.FS
}

// String implements fmt.Stringer
func (a *adapter) String() string {
	return fmt.Sprintf("%v", a.fs)
}

// OpenFile implements FS.OpenFile
func (a *adapter) OpenFile(path string, flag int, perm fs.FileMode) (platform.File, syscall.Errno) {
	path = cleanPath(path)
	f, err := a.fs.Open(path)
	return platform.NewFsFile(path, flag, f), platform.UnwrapOSError(err)
}

// Stat implements FS.Stat
func (a *adapter) Stat(path string) (platform.Stat_t, syscall.Errno) {
	name := cleanPath(path)
	f, err := a.fs.Open(name)
	if err != nil {
		return platform.Stat_t{}, platform.UnwrapOSError(err)
	}
	defer f.Close()
	return platform.NewFsFile(path, syscall.O_RDONLY, f).Stat()
}

// Lstat implements FS.Lstat
func (a *adapter) Lstat(path string) (platform.Stat_t, syscall.Errno) {
	// At this time, we make the assumption that fs.FS instances do not support
	// symbolic links, therefore Lstat is the same as Stat. This is obviously
	// not true but until fs.FS has a solid story for how to handle symlinks we
	// are better off not making a decision that would be difficult to revert
	// later on.
	//
	// For further discussions on the topic, see:
	// https://github.com/golang/go/issues/49580
	return a.Stat(path)
}

func cleanPath(name string) string {
	if len(name) == 0 {
		return name
	}
	// fs.ValidFile cannot be rooted (start with '/')
	cleaned := name
	if name[0] == '/' {
		cleaned = name[1:]
	}
	cleaned = path.Clean(cleaned) // e.g. "sub/." -> "sub"
	return cleaned
}
