package sysfs

import (
	"io/fs"
	"os"
	"syscall"
	"time"

	"github.com/tetratelabs/wazero/internal/platform"
)

// NewReadFS is used to mask an existing FS for reads. Notably, this allows
// the CLI to do read-only mounts of directories the host user can write, but
// doesn't want the guest wasm to. For example, Python libraries shouldn't be
// written to at runtime by the python wasm file.
func NewReadFS(fs FS) FS {
	if _, ok := fs.(*readFS); ok {
		return fs
	} else if _, ok = fs.(UnimplementedFS); ok {
		return fs // unimplemented is read-only
	}
	return &readFS{fs: fs}
}

type readFS struct {
	fs FS
}

// String implements fmt.Stringer
func (r *readFS) String() string {
	return r.fs.String()
}

// OpenFile implements FS.OpenFile
func (r *readFS) OpenFile(path string, flag int, perm fs.FileMode) (platform.File, syscall.Errno) {
	// TODO: Once the real implementation is complete, move the below to
	// /RATIONALE.md. Doing this while the type is unstable creates
	// documentation drift as we expect a lot of reshaping meanwhile.
	//
	// Callers of this function expect to either open a valid file handle, or
	// get an error, if they can't. We want to return ENOSYS if opened for
	// anything except reads.
	//
	// Instead, we could return a fake no-op file on O_WRONLY. However, this
	// hurts observability because a later write error to that file will be on
	// a different source code line than the root cause which is opening with
	// an unsupported flag.
	//
	// The tricky part is os.RD_ONLY is typically defined as zero, so while the
	// parameter is named flag, the part about opening read vs write isn't a
	// typical bitflag. We can't compare against zero anyway, because even if
	// there isn't a current flag to OR in with that, there may be in the
	// future. What we do instead is mask the flags about read/write mode and
	// check if they are the opposite of read or not.
	switch flag & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR) {
	case os.O_WRONLY, os.O_RDWR:
		return nil, syscall.ENOSYS
	default: // os.O_RDONLY so we are ok!
	}

	f, errno := r.fs.OpenFile(path, flag, perm)
	if errno != 0 {
		return nil, errno
	}
	return &readFile{f: f}, 0
}

// compile-time check to ensure readFile implements platform.File.
var _ platform.File = (*readFile)(nil)

type readFile struct {
	f platform.File
}

// Path implements the same method as documented on platform.File.
func (r *readFile) Path() string {
	return r.f.Path()
}

// AccessMode implements the same method as documented on platform.File.
func (r *readFile) AccessMode() int {
	return r.f.AccessMode()
}

// IsNonblock implements the same method as documented on platform.File.
func (r *readFile) IsNonblock() bool {
	return r.f.IsNonblock()
}

// SetNonblock implements the same method as documented on platform.File.
func (r *readFile) SetNonblock(enabled bool) syscall.Errno {
	return r.f.SetNonblock(enabled)
}

// Stat implements the same method as documented on platform.File.
func (r *readFile) Stat() (platform.Stat_t, syscall.Errno) {
	return r.f.Stat()
}

// IsDir implements the same method as documented on platform.File.
func (r *readFile) IsDir() (bool, syscall.Errno) {
	return r.f.IsDir()
}

// Read implements the same method as documented on platform.File.
func (r *readFile) Read(buf []byte) (int, syscall.Errno) {
	return r.f.Read(buf)
}

// Pread implements the same method as documented on platform.File.
func (r *readFile) Pread(buf []byte, offset int64) (int, syscall.Errno) {
	return r.f.Pread(buf, offset)
}

// Seek implements the same method as documented on platform.File.
func (r *readFile) Seek(offset int64, whence int) (int64, syscall.Errno) {
	return r.f.Seek(offset, whence)
}

// Readdir implements the same method as documented on platform.File.
func (r *readFile) Readdir(n int) (dirents []platform.Dirent, errno syscall.Errno) {
	return r.f.Readdir(n)
}

// Write implements the same method as documented on platform.File.
func (r *readFile) Write([]byte) (int, syscall.Errno) {
	return 0, r.writeErr()
}

// Pwrite implements the same method as documented on platform.File.
func (r *readFile) Pwrite([]byte, int64) (n int, errno syscall.Errno) {
	return 0, r.writeErr()
}

// Truncate implements the same method as documented on platform.File.
func (r *readFile) Truncate(int64) syscall.Errno {
	return r.writeErr()
}

// Sync implements the same method as documented on platform.File.
func (r *readFile) Sync() syscall.Errno {
	return syscall.EBADF
}

// Datasync implements the same method as documented on platform.File.
func (r *readFile) Datasync() syscall.Errno {
	return syscall.EBADF
}

// Chmod implements the same method as documented on platform.File.
func (r *readFile) Chmod(fs.FileMode) syscall.Errno {
	return syscall.EBADF
}

// Chown implements the same method as documented on platform.File.
func (r *readFile) Chown(int, int) syscall.Errno {
	return syscall.EBADF
}

// Utimens implements the same method as documented on platform.File.
func (r *readFile) Utimens(*[2]syscall.Timespec) syscall.Errno {
	return syscall.EBADF
}

func (r *readFile) writeErr() syscall.Errno {
	if isDir, errno := r.IsDir(); errno != 0 {
		return errno
	} else if isDir {
		return syscall.EISDIR
	}
	return syscall.EBADF
}

// Close implements the same method as documented on platform.File.
func (r *readFile) Close() syscall.Errno {
	return r.f.Close()
}

// PollRead implements File.PollRead
func (r *readFile) PollRead(timeout *time.Duration) (ready bool, errno syscall.Errno) {
	return r.f.PollRead(timeout)
}

// Lstat implements FS.Lstat
func (r *readFS) Lstat(path string) (platform.Stat_t, syscall.Errno) {
	return r.fs.Lstat(path)
}

// Stat implements FS.Stat
func (r *readFS) Stat(path string) (platform.Stat_t, syscall.Errno) {
	return r.fs.Stat(path)
}

// Readlink implements FS.Readlink
func (r *readFS) Readlink(path string) (dst string, err syscall.Errno) {
	return r.fs.Readlink(path)
}

// Mkdir implements FS.Mkdir
func (r *readFS) Mkdir(path string, perm fs.FileMode) syscall.Errno {
	return syscall.EROFS
}

// Chmod implements FS.Chmod
func (r *readFS) Chmod(path string, perm fs.FileMode) syscall.Errno {
	return syscall.EROFS
}

// Chown implements FS.Chown
func (r *readFS) Chown(path string, uid, gid int) syscall.Errno {
	return syscall.EROFS
}

// Lchown implements FS.Lchown
func (r *readFS) Lchown(path string, uid, gid int) syscall.Errno {
	return syscall.EROFS
}

// Rename implements FS.Rename
func (r *readFS) Rename(from, to string) syscall.Errno {
	return syscall.EROFS
}

// Rmdir implements FS.Rmdir
func (r *readFS) Rmdir(path string) syscall.Errno {
	return syscall.EROFS
}

// Link implements FS.Link
func (r *readFS) Link(_, _ string) syscall.Errno {
	return syscall.EROFS
}

// Symlink implements FS.Symlink
func (r *readFS) Symlink(_, _ string) syscall.Errno {
	return syscall.EROFS
}

// Unlink implements FS.Unlink
func (r *readFS) Unlink(path string) syscall.Errno {
	return syscall.EROFS
}

// Utimens implements FS.Utimens
func (r *readFS) Utimens(path string, times *[2]syscall.Timespec, symlinkFollow bool) syscall.Errno {
	return syscall.EROFS
}

// Truncate implements FS.Truncate
func (r *readFS) Truncate(string, int64) syscall.Errno {
	return syscall.EROFS
}
