package advisor

import (
	"fmt"
	"sort"

	"github.com/zouuup/memadvise/internal/output"
	"github.com/zouuup/memadvise/internal/syscall"
)

// Advisor handles memory advice operations
type Advisor struct {
	pid     int
	regions []syscall.MemoryRegion
	output  *output.OutputManager
}

// New creates a new Advisor
func New(pid int, regions []syscall.MemoryRegion, out *output.OutputManager) *Advisor {
	return &Advisor{
		pid:     pid,
		regions: regions,
		output:  out,
	}
}

// Execute performs the memory advice operation
func (a *Advisor) Execute(budget int64, mode string) error {
	if len(a.regions) == 0 {
		return fmt.Errorf("no eligible memory regions found")
	}

	// First, check if the syscall is supported
	if !syscall.SupportsProcessMadvise() {
		return fmt.Errorf("process_madvise syscall is not supported on this system")
	}

	// Sort regions by size (largest first) for better efficiency
	sortedRegions := make([]syscall.MemoryRegion, len(a.regions))
	copy(sortedRegions, a.regions)
	sort.Slice(sortedRegions, func(i, j int) bool {
		return sortedRegions[i].Size > sortedRegions[j].Size
	})

	// Select regions to advise, up to the budget
	var selectedRegions []syscall.MemoryRegion
	var totalBytes uint64

	for _, region := range sortedRegions {
		if int64(totalBytes) >= budget {
			break
		}

		selectedRegions = append(selectedRegions, region)
		totalBytes += region.Size

		if a.output.IsVerbose() {
			a.output.SelectedRegion(a.pid, region)
		}
	}

	// Apply the advice
	bytesAdvised, err := syscall.ProcessMadvise(a.pid, selectedRegions, mode)
	if err != nil {
		return fmt.Errorf("failed to apply memory advice: %w", err)
	}

	a.output.SummaryResults(a.pid, bytesAdvised, int64(totalBytes), len(selectedRegions), mode)
	return nil
}
