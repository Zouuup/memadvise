package syscall

import (
	"os"
	"testing"
)

func TestPidExists(t *testing.T) {
	// Current process should exist
	currentPID := os.Getpid()

	_, err := OpenPidfd(currentPID)
	if err != nil {
		t.Errorf("OpenPidfd(%d) failed: %v", currentPID, err)
	}

	// Invalid PID should not exist
	invalidPID := 999999999
	_, err = OpenPidfd(invalidPID)
	if err == nil {
		t.Errorf("OpenPidfd(%d) should have failed for non-existent PID", invalidPID)
	}
}

func TestMemoryRegion(t *testing.T) {
	region := MemoryRegion{
		Start:      0x1000,
		End:        0x2000,
		Size:       0x1000,
		Prot:       "rw-p",
		Anonymous:  true,
		Private:    true,
		Writable:   true,
		Executable: false,
		Path:       "[anon]",
	}

	if region.Size != 0x1000 {
		t.Errorf("Expected size 0x1000, got 0x%x", region.Size)
	}

	if !region.Anonymous {
		t.Errorf("Expected region to be anonymous")
	}

	if !region.Private {
		t.Errorf("Expected region to be private")
	}

	if !region.Writable {
		t.Errorf("Expected region to be writable")
	}

	if region.Executable {
		t.Errorf("Expected region not to be executable")
	}
}
