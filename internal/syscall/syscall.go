package syscall

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Constants for madvise modes and syscall numbers
const (
	MADV_COLD    = 20
	MADV_PAGEOUT = 21

	SYS_PIDFD_OPEN      = 434 // syscall number for pidfd_open
	SYS_PROCESS_MADVISE = 440 // syscall number for process_madvise
)

// Iovec represents the structure passed to the process_madvise syscall
type Iovec struct {
	Base uintptr
	Len  uint64
}

// MemoryRegion represents a mapped memory region with its properties
type MemoryRegion struct {
	Start      uint64
	End        uint64
	Size       uint64
	Prot       string
	Anonymous  bool
	Private    bool
	Writable   bool
	Executable bool
	Path       string
}

// OpenPidfd opens a file descriptor for the specified process
func OpenPidfd(pid int) (int, error) {
	// Check if the process exists first
	procPath := fmt.Sprintf("/proc/%d", pid)
	_, err := os.Stat(procPath)
	if err != nil {
		return -1, fmt.Errorf("process %d does not exist: %w", pid, err)
	}

	// Direct syscall for pidfd_open
	r1, _, errno := syscall.Syscall(SYS_PIDFD_OPEN, uintptr(pid), 0, 0)
	if errno != 0 {
		return -1, fmt.Errorf("failed to open pidfd for process %d: %w", pid, errno)
	}

	return int(r1), nil
}

// ProcessMadvise applies memory advice to specified regions
func ProcessMadvise(pid int, regions []MemoryRegion, mode string) (int64, error) {
	pidfd, err := OpenPidfd(pid)
	if err != nil {
		return 0, err
	}
	defer syscall.Close(pidfd)

	// Map mode string to syscall constant
	var adviceVal int
	switch mode {
	case "cold":
		adviceVal = MADV_COLD
	case "pageout":
		adviceVal = MADV_PAGEOUT
	default:
		return 0, fmt.Errorf("invalid mode: %s", mode)
	}

	// Create iovecs from memory regions
	iovecs := make([]Iovec, 0, len(regions))
	for _, region := range regions {
		iovec := Iovec{
			Base: uintptr(region.Start),
			Len:  uint64(region.End - region.Start),
		}
		iovecs = append(iovecs, iovec)
	}

	// Apply the advice directly using the syscall
	r1, _, errno := syscall.Syscall6(
		SYS_PROCESS_MADVISE,
		uintptr(pidfd),
		uintptr(unsafe.Pointer(&iovecs[0])),
		uintptr(len(iovecs)),
		uintptr(adviceVal),
		0,
		0,
	)

	if errno != 0 {
		return 0, fmt.Errorf("process_madvise syscall failed: %w", errno)
	}

	return int64(r1), nil
}

// SupportsProcessMadvise checks if the system supports the process_madvise syscall
func SupportsProcessMadvise() bool {
	// Try opening a pidfd for the current process
	pidfd, err := OpenPidfd(os.Getpid())
	if err != nil {
		return false
	}
	defer syscall.Close(pidfd)

	// Create a dummy iovec for testing
	data := make([]byte, unix.Getpagesize())
	iovec := []Iovec{
		{
			Base: uintptr(unsafe.Pointer(&data[0])),
			Len:  uint64(len(data)),
		},
	}

	// Try the syscall
	_, _, errno := syscall.Syscall6(
		SYS_PROCESS_MADVISE,
		uintptr(pidfd),
		uintptr(unsafe.Pointer(&iovec[0])),
		uintptr(len(iovec)),
		uintptr(MADV_COLD),
		0,
		0,
	)

	return errno == 0
}
