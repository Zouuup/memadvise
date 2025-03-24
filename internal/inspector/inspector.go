package inspector

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/zouuup/memadvise/internal/syscall"
)

// MemoryStats contains information about memory usage for a process
type MemoryStats struct {
	TotalRSS   int64 // Total resident memory in bytes
	TotalSwap  int64 // Total swap usage in bytes
	TotalSize  int64 // Total virtual memory size
	Shared     int64 // Shared memory
	Private    int64 // Private memory
	Anon       int64 // Anonymous memory
	LazyFree   int64 // Lazily freed memory
	SwapPSS    int64 // Proportional swap usage
	HugetlbRSS int64 // Huge pages resident memory
}

// ProcessInspector provides methods to inspect a process's memory
type ProcessInspector struct {
	pid int
}

// NewProcessInspector creates a new process inspector for the given PID
func NewProcessInspector(pid int) (*ProcessInspector, error) {
	// Verify PID exists
	if !PidExists(pid) {
		return nil, fmt.Errorf("process %d does not exist or is not accessible", pid)
	}

	return &ProcessInspector{pid: pid}, nil
}

// PidExists checks if a process with the given PID exists and is accessible
func PidExists(pid int) bool {
	procPath := fmt.Sprintf("/proc/%d", pid)
	_, err := os.Stat(procPath)
	return err == nil
}

// GetMemoryStats retrieves memory statistics for the process
func (p *ProcessInspector) GetMemoryStats() (*MemoryStats, error) {
	stats := &MemoryStats{}

	// Read smaps_rollup
	smapsRollupPath := fmt.Sprintf("/proc/%d/smaps_rollup", p.pid)
	file, err := os.Open(smapsRollupPath)
	if err != nil {
		// Fallback to status if smaps_rollup doesn't exist
		return p.getMemoryStatsFromStatus()
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := parts[0]
		value, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}

		// Convert from KB to bytes
		value *= 1024

		switch key {
		case "Rss:":
			stats.TotalRSS = value
		case "Swap:":
			stats.TotalSwap = value
		case "Size:":
			stats.TotalSize = value
		case "Shared_Clean:", "Shared_Dirty:":
			stats.Shared += value
		case "Private_Clean:", "Private_Dirty:":
			stats.Private += value
		case "Anonymous:":
			stats.Anon = value
		case "LazyFree:":
			stats.LazyFree = value
		case "SwapPss:":
			stats.SwapPSS = value
		case "HugetlbRss:":
			stats.HugetlbRSS = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading smaps_rollup: %w", err)
	}

	return stats, nil
}

// getMemoryStatsFromStatus is a fallback method to get memory stats from /proc/[pid]/status
func (p *ProcessInspector) getMemoryStatsFromStatus() (*MemoryStats, error) {
	stats := &MemoryStats{}

	statusPath := fmt.Sprintf("/proc/%d/status", p.pid)
	file, err := os.Open(statusPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open status file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := parts[0]
		var value int64

		if len(parts) >= 3 && (parts[2] == "kB" || parts[2] == "KB") {
			val, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				continue
			}
			value = val * 1024 // Convert from KB to bytes
		} else {
			val, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				continue
			}
			value = val
		}

		switch key {
		case "VmRSS:":
			stats.TotalRSS = value
		case "VmSwap:":
			stats.TotalSwap = value
		case "VmSize:":
			stats.TotalSize = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading status file: %w", err)
	}

	return stats, nil
}

// GetEligibleRegions returns memory regions eligible for memory advice
func (p *ProcessInspector) GetEligibleRegions() ([]syscall.MemoryRegion, error) {
	var regions []syscall.MemoryRegion

	// Read /proc/[pid]/maps
	mapsPath := fmt.Sprintf("/proc/%d/maps", p.pid)
	file, err := os.Open(mapsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open maps file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		region, err := parseMapLine(line)
		if err != nil {
			continue // Skip lines we can't parse
		}

		// Filter for eligible regions: anonymous, private, writable
		if region.Anonymous && region.Private && region.Writable {
			// Exclude certain regions
			if isExcludedRegion(region) {
				continue
			}
			regions = append(regions, region)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading maps file: %w", err)
	}

	return regions, nil
}

// parseMapLine parses a line from /proc/[pid]/maps
func parseMapLine(line string) (syscall.MemoryRegion, error) {
	var region syscall.MemoryRegion

	// Parse address range
	parts := strings.Fields(line)
	if len(parts) < 5 {
		return region, fmt.Errorf("invalid maps line format: %s", line)
	}

	// Parse address range (first field, in format "start-end")
	addrRange := strings.Split(parts[0], "-")
	if len(addrRange) != 2 {
		return region, fmt.Errorf("invalid address range format: %s", parts[0])
	}

	start, err := strconv.ParseUint(addrRange[0], 16, 64)
	if err != nil {
		return region, fmt.Errorf("invalid start address: %s", addrRange[0])
	}

	end, err := strconv.ParseUint(addrRange[1], 16, 64)
	if err != nil {
		return region, fmt.Errorf("invalid end address: %s", addrRange[1])
	}

	// Parse permissions (second field, in format "rwxp")
	perms := parts[1]
	if len(perms) < 4 {
		return region, fmt.Errorf("invalid permissions format: %s", perms)
	}

	// Last part is the path, if any
	path := ""
	if len(parts) >= 6 {
		path = parts[5]
	}

	region = syscall.MemoryRegion{
		Start:      start,
		End:        end,
		Size:       end - start,
		Prot:       perms,
		Writable:   strings.Contains(perms, "w"),
		Executable: strings.Contains(perms, "x"),
		Private:    strings.Contains(perms, "p"),
		Anonymous:  path == "" || path == "[anon]" || path == "[heap]" || strings.HasPrefix(path, "[stack"),
		Path:       path,
	}

	return region, nil
}

// isExcludedRegion checks if a memory region should be excluded from advising
func isExcludedRegion(region syscall.MemoryRegion) bool {
	// Exclude stack regions
	if strings.HasPrefix(region.Path, "[stack") {
		return true
	}

	// Exclude vdso, vvar
	if region.Path == "[vdso]" || region.Path == "[vvar]" {
		return true
	}

	// Exclude executable regions
	if region.Executable {
		return true
	}

	// Exclude small regions (less than 4KB)
	if region.Size < 4096 {
		return true
	}

	return false
}
