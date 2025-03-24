package inspector

import (
	"testing"

	"github.com/zouuup/memadvise/internal/syscall"
)

func TestParseMapLine(t *testing.T) {
	testCases := []struct {
		name       string
		line       string
		wantRegion bool
		anonymous  bool
		private    bool
		writable   bool
		executable bool
	}{
		{
			name:       "anonymous private writable heap",
			line:       "00400000-00401000 rw-p 00000000 00:00 0                      [heap]",
			wantRegion: true,
			anonymous:  true,
			private:    true,
			writable:   true,
			executable: false,
		},
		{
			name:       "executable code region",
			line:       "00600000-00601000 r-xp 00000000 08:01 123456                 /usr/bin/example",
			wantRegion: true,
			anonymous:  false,
			private:    true,
			writable:   false,
			executable: true,
		},
		{
			name:       "shared library",
			line:       "7f8cc09a7000-7f8cc09c9000 r--p 00000000 08:01 123456        /usr/lib/libc.so.6",
			wantRegion: true,
			anonymous:  false,
			private:    true,
			writable:   false,
			executable: false,
		},
		{
			name:       "stack region",
			line:       "7ffe2bd37000-7ffe2bd58000 rw-p 00000000 00:00 0             [stack]",
			wantRegion: true,
			anonymous:  true,
			private:    true,
			writable:   true,
			executable: false,
		},
		{
			name:       "invalid line",
			line:       "invalid line format",
			wantRegion: false,
			anonymous:  false,
			private:    false,
			writable:   false,
			executable: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			region, err := parseMapLine(tc.line)
			if !tc.wantRegion {
				if err == nil {
					t.Errorf("parseMapLine() expected error for invalid line, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("parseMapLine() unexpected error: %v", err)
				return
			}

			if region.Anonymous != tc.anonymous {
				t.Errorf("parseMapLine() anonymous = %v, want %v", region.Anonymous, tc.anonymous)
			}

			if region.Private != tc.private {
				t.Errorf("parseMapLine() private = %v, want %v", region.Private, tc.private)
			}

			if region.Writable != tc.writable {
				t.Errorf("parseMapLine() writable = %v, want %v", region.Writable, tc.writable)
			}

			if region.Executable != tc.executable {
				t.Errorf("parseMapLine() executable = %v, want %v", region.Executable, tc.executable)
			}
		})
	}
}

func TestIsExcludedRegion(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		size     uint64
		exec     bool
		excluded bool
	}{
		{
			name:     "stack region",
			path:     "[stack]",
			size:     4096,
			exec:     false,
			excluded: true,
		},
		{
			name:     "vdso region",
			path:     "[vdso]",
			size:     4096,
			exec:     false,
			excluded: true,
		},
		{
			name:     "executable region",
			path:     "/bin/bash",
			size:     4096,
			exec:     true,
			excluded: true,
		},
		{
			name:     "small region",
			path:     "[anon]",
			size:     2048, // less than 4KB
			exec:     false,
			excluded: true,
		},
		{
			name:     "valid region",
			path:     "[anon]",
			size:     8192,
			exec:     false,
			excluded: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			region := syscall.MemoryRegion{
				Path:       tc.path,
				Size:       tc.size,
				Executable: tc.exec,
			}

			excluded := isExcludedRegion(region)

			if excluded != tc.excluded {
				t.Errorf("isExcludedRegion() = %v, want %v", excluded, tc.excluded)
			}
		})
	}
}
