package platform

import (
	"io"
	"io/fs"
	"syscall"
	"time"
)

// File is a writeable fs.File bridge backed by syscall functions needed for ABI
// including WASI and runtime.GOOS=js.
//
// Implementations should embed UnimplementedFile for forward compatability. Any
// unsupported method or parameter should return syscall.ENOSYS.
//
// # Errors
//
// All methods that can return an error return a syscall.Errno, which is zero
// on success.
//
// Restricting to syscall.Errno matches current WebAssembly host functions,
// which are constrained to well-known error codes. For example, `GOOS=js` maps
// hard coded values and panics otherwise. More commonly, WASI maps syscall
// errors to u32 numeric values.
//
// # Notes
//
// A writable filesystem abstraction is not yet implemented as of Go 1.20. See
// https://github.com/golang/go/issues/45757
type File interface {
	// Path returns path used to open the file or empty if not applicable. For
	// example, a file representing stdout will return empty.
	//
	// Note: This can drift on rename.
	Path() string

	// AccessMode returns the access mode the file was opened with.
	//
	// This returns exclusively one of the following:
	//   - syscall.O_RDONLY: read-only, e.g. os.Stdin
	//   - syscall.O_WRONLY: write-only, e.g. os.Stdout
	//   - syscall.O_RDWR: read-write, e.g. os.CreateTemp
	AccessMode() int

	// IsNonblock returns true if SetNonblock was successfully enabled on this
	// file.
	//
	// # Notes
	//
	//   - This may not match the underlying state of the file descriptor if it
	//     was opened (OpenFile) in non-blocking mode.
	IsNonblock() bool
	// ^-- TODO: We should be able to cache the open flag and remove this note.

	// SetNonblock toggles the non-blocking mode of this file.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.ENOSYS: the implementation does not support this function.
	//   - syscall.EBADF: the file or directory was closed.
	//
	// # Notes
	//
	//   - This is like syscall.SetNonblock and `fcntl` with `O_NONBLOCK` in
	//     POSIX. See https://pubs.opengroup.org/onlinepubs/9699919799/functions/fcntl.html
	SetNonblock(enable bool) syscall.Errno

	// Stat is similar to syscall.Fstat.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.ENOSYS: the implementation does not support this function.
	//   - syscall.EBADF: the file or directory was closed.
	//
	// # Notes
	//
	//   - This is like syscall.Fstat and `fstatat` with `AT_FDCWD` in POSIX.
	//     See https://pubs.opengroup.org/onlinepubs/9699919799/functions/stat.html
	//   - A fs.FileInfo backed implementation sets atim, mtim and ctim to the
	//     same value.
	//   - Windows allows you to stat a closed directory.
	Stat() (Stat_t, syscall.Errno)

	// IsDir returns true if this file is a directory or an error there was an
	// error retrieving this information.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.ENOSYS: the implementation does not support this function.
	//
	// # Notes
	//
	//   - Some implementations implement this with a cached call to Stat.
	IsDir() (bool, syscall.Errno)

	// Read attempts to read all bytes in the file into `buf`, and returns the
	// count read even on error.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.ENOSYS: the implementation does not support this function.
	//   - syscall.EBADF: the file or directory was closed or not readable.
	//   - syscall.EISDIR: the file was a directory.
	//
	// # Notes
	//
	//   - This is like io.Reader and `read` in POSIX, preferring semantics of
	//     io.Reader. See https://pubs.opengroup.org/onlinepubs/9699919799/functions/read.html
	//   - Unlike io.Reader, there is no io.EOF returned on end-of-file. To
	//     read the file completely, the caller must repeat until `n` is zero.
	Read(buf []byte) (n int, errno syscall.Errno)

	// Pread attempts to read all bytes in the file into `p`, starting at the
	// offset `off`, and returns the count read even on error.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.ENOSYS: the implementation does not support this function.
	//   - syscall.EBADF: the file or directory was closed or not readable.
	//   - syscall.EINVAL: the offset was negative.
	//   - syscall.EISDIR: the file was a directory.
	//
	// # Notes
	//
	//   - This is like io.ReaderAt and `pread` in POSIX, preferring semantics
	//     of io.ReaderAt. See https://pubs.opengroup.org/onlinepubs/9699919799/functions/pread.html
	//   - Unlike io.ReaderAt, there is no io.EOF returned on end-of-file. To
	//     read the file completely, the caller must repeat until `n` is zero.
	Pread(p []byte, off int64) (n int, errno syscall.Errno)

	// Seek attempts to set the next offset for Read or Write and returns the
	// resulting absolute offset or an error.
	//
	// # Parameters
	//
	// The `offset` parameters is interpreted in terms of `whence`:
	//   - io.SeekStart: relative to the start of the file, e.g. offset=0 sets
	//     the next Read or Write to the beginning of the file.
	//   - io.SeekCurrent: relative to the current offset, e.g. offset=16 sets
	//     the next Read or Write 16 bytes past the prior.
	//   - io.SeekEnd: relative to the end of the file, e.g. offset=-1 sets the
	//     next Read or Write to the last byte in the file.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.ENOSYS: the implementation does not support this function.
	//   - syscall.EBADF: the file or directory was closed or not readable.
	//   - syscall.EINVAL: the offset was negative.
	//   - syscall.EISDIR: the file was a directory.
	//
	// # Notes
	//
	//   - This is like io.Seeker and `fseek` in POSIX, preferring semantics
	//     of io.Seeker. See https://pubs.opengroup.org/onlinepubs/9699919799/functions/fseek.html
	Seek(offset int64, whence int) (newOffset int64, errno syscall.Errno)

	// PollRead returns if the file has data ready to be read or an error.
	//
	// # Parameters
	//
	// The `timeout` parameter when nil blocks up to forever.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.ENOSYS: the implementation does not support this function.
	//
	// # Notes
	//
	//   - This is like `poll` in POSIX, for a single file.
	//     See https://pubs.opengroup.org/onlinepubs/9699919799/functions/poll.html
	//   - No-op files, such as those which read from /dev/null, should return
	//     immediately true to avoid hangs (because data will never become
	//     available).
	PollRead(timeout *time.Duration) (ready bool, errno syscall.Errno)

	// Readdir reads the contents of the directory associated with file and
	// returns a slice of up to n Dirent values in an arbitrary order. This is
	// a stateful function, so subsequent calls return any next values.
	//
	// If n > 0, Readdir returns at most n entries or an error.
	// If n <= 0, Readdir returns all remaining entries or an error.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.ENOSYS: the implementation does not support this function.
	//   - syscall.ENOTDIR: the file was not a directory
	//
	// # Notes
	//
	//   - This is like `Readdir` on os.File, but unlike `readdir` in POSIX.
	//     See https://pubs.opengroup.org/onlinepubs/9699919799/functions/readdir.html
	//   - For portability reasons, no error is returned at the end of the
	//     directory, when the file is closed or removed while open.
	//     See https://github.com/ziglang/zig/blob/0.10.1/lib/std/fs.zig#L635-L637
	Readdir(n int) (dirents []Dirent, errno syscall.Errno)
	// ^-- TODO: consider being more like POSIX, for example, returning a
	// closeable Dirent object that can iterate on demand. This would
	// centralize sizing logic needed by wasi, particularly extra dirents
	// stored in the sys.FileEntry type. It could possibly reduce the need to
	// reopen the whole file.

	// Write attempts to write all bytes in `p` to the file, and returns the
	// count written even on error.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.ENOSYS: the implementation does not support this function.
	//   - syscall.EBADF: the file or directory was closed or not writeable.
	//   - syscall.EISDIR: the file was a directory.
	//
	// # Notes
	//
	//   - This is like io.Writer and `write` in POSIX, preferring semantics of
	//     io.Writer. See https://pubs.opengroup.org/onlinepubs/9699919799/functions/write.html
	Write(p []byte) (n int, errno syscall.Errno)

	// Pwrite attempts to write all bytes in `p` to the file at the given
	// offset `off`, and returns the count written even on error.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.ENOSYS: the implementation does not support this function.
	//   - syscall.EBADF: the file or directory was closed or not writeable.
	//   - syscall.EINVAL: the offset was negative.
	//   - syscall.EISDIR: the file was a directory.
	//
	// # Notes
	//
	//   - This is like io.WriterAt and `pwrite` in POSIX, preferring semantics
	//     of io.WriterAt. See https://pubs.opengroup.org/onlinepubs/9699919799/functions/pwrite.html
	Pwrite(p []byte, off int64) (n int, errno syscall.Errno)

	// Truncate truncates a file to a specified length.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.ENOSYS: the implementation does not support this function.
	//   - syscall.EBADF: the file or directory was closed.
	//   - syscall.EINVAL: the `size` is negative.
	//   - syscall.EISDIR: the file was a directory.
	//
	// # Notes
	//
	//   - This is like syscall.Ftruncate and `ftruncate` in POSIX. See
	//     https://pubs.opengroup.org/onlinepubs/9699919799/functions/ftruncate.html
	//   - Windows does not error when calling Truncate on a closed file.
	Truncate(size int64) syscall.Errno

	// Sync synchronizes changes to the file.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.EBADF: the file or directory was closed.
	//
	// # Notes
	//
	//   - This is like syscall.Fsync and `fsync` in POSIX. See
	//     https://pubs.opengroup.org/onlinepubs/9699919799/functions/fsync.html
	//   - This returns with no error instead of syscall.ENOSYS when
	//     unimplemented. This prevents fake filesystems from erring.
	//   - Windows does not error when calling Sync on a closed file.
	Sync() syscall.Errno

	// Datasync synchronizes the data of a file.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.EBADF: the file or directory was closed.
	//
	// # Notes
	//
	//   - This is like syscall.Fdatasync and `fdatasync` in POSIX. See
	//     https://pubs.opengroup.org/onlinepubs/9699919799/functions/fdatasync.html
	//   - This returns with no error instead of syscall.ENOSYS when
	//     unimplemented. This prevents fake filesystems from erring.
	//   - As this is commonly missing, some implementations dispatch to Sync.
	Datasync() syscall.Errno

	// Chmod changes the mode of the file.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.ENOSYS: the implementation does not support this function.
	//   - syscall.EBADF: the file or directory was closed.
	//
	// # Notes
	//
	//   - This is like syscall.Fchmod and `fchmod` in POSIX. See
	//     https://pubs.opengroup.org/onlinepubs/9699919799/functions/fchmod.html
	//   - Windows ignores the execute bit, and any permissions come back as
	//     group and world. For example, chmod of 0400 reads back as 0444, and
	//     0700 0666. Also, permissions on directories aren't supported at all.
	Chmod(fs.FileMode) syscall.Errno

	// Chown changes the owner and group of a file.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.ENOSYS: the implementation does not support this function.
	//   - syscall.EBADF: the file or directory was closed.
	//
	// # Notes
	//
	//   - This is like syscall.Fchown and `fchown` in POSIX. See
	//     https://pubs.opengroup.org/onlinepubs/9699919799/functions/fchown.html
	//   - This always returns syscall.ENOSYS on windows.
	Chown(uid, gid int) syscall.Errno

	// Utimens set file access and modification times of this file, at
	// nanosecond precision.
	//
	// # Parameters
	//
	// The `times` parameter includes the access and modification timestamps to
	// assign. Special syscall.Timespec NSec values UTIME_NOW and UTIME_OMIT may be
	// specified instead of real timestamps. A nil `times` parameter behaves the
	// same as if both were set to UTIME_NOW.
	//
	// # Errors
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.ENOSYS: the implementation does not support this function.
	//   - syscall.EBADF: the file or directory was closed.
	//
	// # Notes
	//
	//   - This is like syscall.UtimesNano and `futimens` in POSIX. See
	//     https://pubs.opengroup.org/onlinepubs/9699919799/functions/futimens.html
	//   - Windows requires files to be open with syscall.O_RDWR, which means you
	//     cannot use this to update timestamps on a directory (syscall.EPERM).
	Utimens(times *[2]syscall.Timespec) syscall.Errno

	// Close closes the underlying file.
	//
	// A zero syscall.Errno is success. The below are expected otherwise:
	//   - syscall.ENOSYS: the implementation does not support this function.
	//
	// # Notes
	//
	//   - This is like syscall.Close and `close` in POSIX. See
	//     https://pubs.opengroup.org/onlinepubs/9699919799/functions/close.html
	Close() syscall.Errno
}

// UnimplementedFile is a File that returns syscall.ENOSYS for all functions,
// This should be embedded to have forward compatible implementations.
type UnimplementedFile struct{}

// IsNonblock implements File.IsNonblock
func (UnimplementedFile) IsNonblock() bool {
	return false
}

// SetNonblock implements File.SetNonblock
func (UnimplementedFile) SetNonblock(bool) syscall.Errno {
	return syscall.ENOSYS
}

// Stat implements File.Stat
func (UnimplementedFile) Stat() (Stat_t, syscall.Errno) {
	return Stat_t{}, syscall.ENOSYS
}

// IsDir implements File.IsDir
func (UnimplementedFile) IsDir() (bool, syscall.Errno) {
	return false, syscall.ENOSYS
}

// Read implements File.Read
func (UnimplementedFile) Read([]byte) (int, syscall.Errno) {
	return 0, syscall.ENOSYS
}

// Pread implements File.Pread
func (UnimplementedFile) Pread([]byte, int64) (int, syscall.Errno) {
	return 0, syscall.ENOSYS
}

// Seek implements File.Seek
func (UnimplementedFile) Seek(int64, int) (int64, syscall.Errno) {
	return 0, syscall.ENOSYS
}

// Readdir implements File.Readdir
func (UnimplementedFile) Readdir(int) (dirents []Dirent, errno syscall.Errno) {
	return nil, syscall.ENOSYS
}

// PollRead implements File.PollRead
func (UnimplementedFile) PollRead(*time.Duration) (ready bool, errno syscall.Errno) {
	return false, syscall.ENOSYS
}

// Write implements File.Write
func (UnimplementedFile) Write([]byte) (int, syscall.Errno) {
	return 0, syscall.ENOSYS
}

// Pwrite implements File.Pwrite
func (UnimplementedFile) Pwrite([]byte, int64) (int, syscall.Errno) {
	return 0, syscall.ENOSYS
}

// Truncate implements File.Truncate
func (UnimplementedFile) Truncate(int64) syscall.Errno {
	return syscall.ENOSYS
}

// Sync implements File.Sync
func (UnimplementedFile) Sync() syscall.Errno {
	return 0 // not syscall.ENOSYS
}

// Datasync implements File.Datasync
func (UnimplementedFile) Datasync() syscall.Errno {
	return 0 // not syscall.ENOSYS
}

// Chmod implements File.Chmod
func (UnimplementedFile) Chmod(fs.FileMode) syscall.Errno {
	return syscall.ENOSYS
}

// Chown implements File.Chown
func (UnimplementedFile) Chown(int, int) syscall.Errno {
	return syscall.ENOSYS
}

// Utimens implements File.Utimens
func (UnimplementedFile) Utimens(*[2]syscall.Timespec) syscall.Errno {
	return syscall.ENOSYS
}

func NewStdioFile(stdin bool, f fs.File) (File, error) {
	// Return constant stat, which has fake times, but keep the underlying
	// file mode. Fake times are needed to pass wasi-testsuite.
	// https://github.com/WebAssembly/wasi-testsuite/blob/af57727/tests/rust/src/bin/fd_filestat_get.rs#L1-L19
	var mode fs.FileMode
	if st, err := f.Stat(); err != nil {
		return nil, err
	} else {
		mode = st.Mode()
	}
	var accessMode int
	if stdin {
		accessMode = syscall.O_RDONLY
	} else {
		accessMode = syscall.O_WRONLY
	}
	return &stdioFile{
		fsFile: fsFile{accessMode: accessMode, file: f},
		st:     Stat_t{Mode: mode, Nlink: 1},
	}, nil
}

func NewFsFile(openPath string, openFlag int, f fs.File) File {
	return &fsFile{
		path:       openPath,
		accessMode: openFlag & (syscall.O_RDONLY | syscall.O_WRONLY | syscall.O_RDWR),
		file:       f,
	}
}

type stdioFile struct {
	fsFile
	st Stat_t
}

// IsDir implements File.IsDir
func (f *stdioFile) IsDir() (bool, syscall.Errno) {
	return false, 0
}

// Stat implements File.Stat
func (f *stdioFile) Stat() (Stat_t, syscall.Errno) {
	return f.st, 0
}

// Close implements File.Close
func (f *stdioFile) Close() syscall.Errno {
	return 0
}

type fsFile struct {
	path       string
	accessMode int
	file       fs.File

	nonblock bool

	// cachedStat includes fields that won't change while a file is open.
	cachedSt *cachedStat
}

type cachedStat struct {
	// fileType is the same as what's documented on Dirent.
	fileType fs.FileMode
}

// cachedStat returns the cacheable parts of platform.Stat_t or an error if
// they couldn't be retrieved.
func (f *fsFile) cachedStat() (fileType fs.FileMode, errno syscall.Errno) {
	if f.cachedSt == nil {
		if _, errno = f.Stat(); errno != 0 {
			return
		}
	}
	return f.cachedSt.fileType, 0
}

// Path implements File.Path
func (f *fsFile) Path() string {
	return f.path
}

// AccessMode implements File.AccessMode
func (f *fsFile) AccessMode() int {
	return f.accessMode
}

// IsNonblock implements File.IsNonblock
func (f *fsFile) IsNonblock() bool {
	return f.nonblock
}

// SetNonblock implements File.SetNonblock
func (f *fsFile) SetNonblock(enable bool) syscall.Errno {
	if fd, ok := f.file.(fdFile); ok {
		if err := setNonblock(fd.Fd(), enable); err != nil {
			return UnwrapOSError(err)
		}
		f.nonblock = enable
		return 0
	}
	return syscall.ENOSYS
}

// IsDir implements File.IsDir
func (f *fsFile) IsDir() (bool, syscall.Errno) {
	if ft, errno := f.cachedStat(); errno != 0 {
		return false, errno
	} else if ft.Type() == fs.ModeDir {
		return true, 0
	}
	return false, 0
}

// Stat implements File.Stat
func (f *fsFile) Stat() (Stat_t, syscall.Errno) {
	st, errno := statFile(f.file)
	switch errno {
	case 0:
		f.cachedSt = &cachedStat{fileType: st.Mode & fs.ModeType}
	case syscall.EIO:
		errno = syscall.EBADF
	}
	return st, errno
}

// Read implements File.Read
func (f *fsFile) Read(p []byte) (n int, errno syscall.Errno) {
	if len(p) == 0 {
		return 0, 0 // less overhead on zero-length reads.
	}

	if errno = f.isDirErrno(); errno != 0 {
		return
	} else if f.accessMode == syscall.O_WRONLY {
		return 0, syscall.EBADF
	}

	if w, ok := f.file.(io.Reader); ok {
		n, err := w.Read(p)
		return n, UnwrapOSError(err)
	}
	return 0, syscall.EBADF
}

// Pread implements File.Pread
func (f *fsFile) Pread(p []byte, off int64) (n int, errno syscall.Errno) {
	if len(p) == 0 {
		return 0, 0 // less overhead on zero-length reads.
	}

	if errno = f.isDirErrno(); errno != 0 {
		return
	} else if f.accessMode == syscall.O_WRONLY {
		return 0, syscall.EBADF
	}

	// Simple case, handle with io.ReaderAt.
	if w, ok := f.file.(io.ReaderAt); ok {
		n, err := w.ReadAt(p, off)
		return n, UnwrapOSError(err)
	}

	// See /RATIONALE.md "fd_pread: io.Seeker fallback when io.ReaderAt is not supported"
	if rs, ok := f.file.(io.ReadSeeker); ok {
		// Determine the current position in the file, as we need to revert it.
		currentOffset, err := rs.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, UnwrapOSError(err)
		}

		// Put the read position back when complete.
		defer func() { _, _ = rs.Seek(currentOffset, io.SeekStart) }()

		// If the current offset isn't in sync with this reader, move it.
		if off != currentOffset {
			if _, err = rs.Seek(off, io.SeekStart); err != nil {
				return 0, UnwrapOSError(err)
			}
		}

		n, err := rs.Read(p)
		return n, UnwrapOSError(err)
	}

	return 0, syscall.ENOSYS // unsupported
}

// Seek implements File.Seek
func (f *fsFile) Seek(offset int64, whence int) (int64, syscall.Errno) {
	if errno := f.isDirErrno(); errno != 0 {
		return 0, errno
	} else if uint(whence) > io.SeekEnd {
		return 0, syscall.EINVAL // negative or exceeds the largest valid whence
	}

	if seeker, ok := f.file.(io.Seeker); ok {
		newOffset, err := seeker.Seek(offset, whence)
		return newOffset, UnwrapOSError(err)
	}
	return 0, syscall.ENOSYS
}

// PollRead implements File.PollRead
func (f *fsFile) PollRead(timeout *time.Duration) (ready bool, errno syscall.Errno) {
	if f, ok := f.file.(fdFile); ok {
		fdSet := FdSet{}
		fd := int(f.Fd())
		fdSet.Set(fd)
		nfds := fd + 1 // See https://man7.org/linux/man-pages/man2/select.2.html#:~:text=condition%20has%20occurred.-,nfds,-This%20argument%20should
		count, err := _select(nfds, &fdSet, nil, nil, timeout)
		return count > 0, UnwrapOSError(err)
	}
	return false, syscall.ENOSYS
}

// Readdir implements File.Readdir
func (f *fsFile) Readdir(n int) ([]Dirent, syscall.Errno) {
	if isDir, errno := f.IsDir(); errno != 0 {
		return nil, errno
	} else if !isDir {
		return nil, syscall.ENOTDIR
	}
	return readdir(f.file, n)
}

// Write implements File.Write
func (f *fsFile) Write(p []byte) (n int, errno syscall.Errno) {
	if errno = f.isDirErrno(); errno != 0 {
		return
	} else if f.accessMode == syscall.O_RDONLY {
		return 0, syscall.EBADF
	}

	if len(p) == 0 {
		return 0, 0 // less overhead on zero-length writes.
	}
	if w, ok := f.file.(io.Writer); ok {
		n, err := w.Write(p)
		return n, UnwrapOSError(err)
	}
	return 0, syscall.ENOSYS // unsupported
}

// Pwrite implements File.Pwrite
func (f *fsFile) Pwrite(p []byte, off int64) (n int, errno syscall.Errno) {
	if errno = f.isDirErrno(); errno != 0 {
		return
	} else if f.accessMode == syscall.O_RDONLY {
		return 0, syscall.EBADF
	}

	if len(p) == 0 {
		return 0, 0 // less overhead on zero-length writes.
	}

	if w, ok := f.file.(io.WriterAt); ok {
		n, err := w.WriteAt(p, off)
		return n, UnwrapOSError(err)
	}
	return 0, syscall.ENOSYS // unsupported
}

// Truncate implements File.Truncate
func (f *fsFile) Truncate(size int64) syscall.Errno {
	if errno := f.isDirErrno(); errno != 0 {
		return errno
	} else if f.accessMode == syscall.O_RDONLY {
		return syscall.EBADF
	}

	if tf, ok := f.file.(truncateFile); ok {
		return UnwrapOSError(tf.Truncate(size))
	}
	return syscall.ENOSYS
}

// isDirErrno returns syscall.EISDIR, if the file is a directory, or any error
// calling IsDir.
func (f *fsFile) isDirErrno() syscall.Errno {
	if isDir, errno := f.IsDir(); errno != 0 {
		return errno
	} else if isDir {
		return syscall.EISDIR
	}
	return 0
}

// Sync implements File.Sync
func (f *fsFile) Sync() syscall.Errno {
	return sync(f.file)
}

// Datasync implements File.Datasync
func (f *fsFile) Datasync() syscall.Errno {
	return datasync(f.file)
}

// Chmod implements File.Chmod
func (f *fsFile) Chmod(mode fs.FileMode) syscall.Errno {
	if f, ok := f.file.(chmodFile); ok {
		return UnwrapOSError(f.Chmod(mode))
	}
	return syscall.ENOSYS
}

// Chown implements File.Chown
func (f *fsFile) Chown(uid, gid int) syscall.Errno {
	if f, ok := f.file.(fdFile); ok {
		return fchown(f.Fd(), uid, gid)
	}
	return syscall.ENOSYS
}

// Utimens implements File.Utimens
func (f *fsFile) Utimens(times *[2]syscall.Timespec) syscall.Errno {
	if f, ok := f.file.(fdFile); ok {
		err := futimens(f.Fd(), times)
		return UnwrapOSError(err)
	}
	return syscall.ENOSYS
}

// Close implements File.Close
func (f *fsFile) Close() syscall.Errno {
	return UnwrapOSError(f.file.Close())
}

// The following interfaces are used until we finalize our own FD-scoped file.
type (
	// PathFile is implemented on files that retain the path to their pre-open.
	PathFile interface {
		Path() string
	}
	// fdFile is implemented by os.File in file_unix.go and file_windows.go
	fdFile interface{ Fd() (fd uintptr) }
	// readdirFile is implemented by os.File in dir.go
	readdirFile interface {
		Readdir(n int) ([]fs.FileInfo, error)
	}
	// chmodFile is implemented by os.File in file_posix.go
	chmodFile interface{ Chmod(fs.FileMode) error }
	// syncFile is implemented by os.File in file_posix.go
	syncFile interface{ Sync() error }
	// truncateFile is implemented by os.File in file_posix.go
	truncateFile interface{ Truncate(size int64) error }
)
