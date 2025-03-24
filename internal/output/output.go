package output

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/zouuup/memadvise/internal/inspector"
	"github.com/zouuup/memadvise/internal/syscall"
)

// OutputManager handles formatted output for the CLI
type OutputManager struct {
	verbose bool
	json    bool
	writer  *tabwriter.Writer
}

// New creates a new OutputManager
func New(verbose bool, jsonOutput bool) *OutputManager {
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	return &OutputManager{
		verbose: verbose,
		json:    jsonOutput,
		writer:  writer,
	}
}

// IsVerbose returns whether verbose output is enabled
func (o *OutputManager) IsVerbose() bool {
	return o.verbose
}

// MemoryStatsBefore outputs memory statistics before advice
func (o *OutputManager) MemoryStatsBefore(pid int, stats *inspector.MemoryStats) {
	if o.json {
		return // Save for final output in JSON mode
	}

	fmt.Fprintf(o.writer, "PID %d Before:\tRSS: %s\tAnon: %s\tPrivate: %s\n",
		pid, formatBytes(stats.TotalRSS), formatBytes(stats.Anon), formatBytes(stats.Private))
	o.writer.Flush()
}

// MemoryStatsAfter outputs memory statistics after advice
func (o *OutputManager) MemoryStatsAfter(pid int, after *inspector.MemoryStats, before *inspector.MemoryStats) {
	if o.json {
		return // Save for final output in JSON mode
	}

	diff := before.TotalRSS - after.TotalRSS
	fmt.Fprintf(o.writer, "PID %d After:\tRSS: %s\tDifference: %s\n",
		pid, formatBytes(after.TotalRSS), formatBytes(diff))
	o.writer.Flush()
}

// SelectedRegion outputs information about a selected memory region
func (o *OutputManager) SelectedRegion(pid int, region syscall.MemoryRegion) {
	if o.json || !o.verbose {
		return // Only in verbose text mode
	}

	path := region.Path
	if path == "" {
		path = "[anon]"
	}

	fmt.Fprintf(o.writer, "PID %d Selected Region:\t%016x-%016x\t%s\t%s\n",
		pid, region.Start, region.End, formatBytes(int64(region.Size)), path)
	o.writer.Flush()
}

// DryRun outputs what would happen in a dry run
func (o *OutputManager) DryRun(pid int, budget int64, mode string, regionCount int) {
	if o.json {
		data := map[string]interface{}{
			"pid":          pid,
			"would_advise": budget,
			"mode":         mode,
			"region_count": regionCount,
			"dry_run":      true,
		}
		o.outputJSON(data)
		return
	}

	fmt.Fprintf(o.writer, "PID %d DRY RUN:\tWould advise %s across %d regions using mode '%s'\n",
		pid, formatBytes(budget), regionCount, mode)
	o.writer.Flush()
}

// SummaryResults outputs summary results after applying advice
func (o *OutputManager) SummaryResults(pid int, bytesAdvised int64, bytesSelected int64, regionCount int, mode string) {
	if o.json {
		data := map[string]interface{}{
			"pid":            pid,
			"advised_bytes":  bytesAdvised,
			"selected_bytes": bytesSelected,
			"regions":        regionCount,
			"mode":           mode,
		}
		o.outputJSON(data)
		return
	}

	fmt.Fprintf(o.writer, "PID %d Summary:\tAdvised %s / %s (%d%%) across %d regions using mode '%s'\n",
		pid, formatBytes(bytesAdvised), formatBytes(bytesSelected),
		int(bytesAdvised*100/bytesSelected), regionCount, mode)
	o.writer.Flush()
}

// Error outputs an error message
func (o *OutputManager) Error(msg string) {
	if o.json {
		data := map[string]interface{}{
			"error": msg,
		}
		o.outputJSON(data)
		return
	}

	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
}

// outputJSON marshals and outputs JSON data
func (o *OutputManager) outputJSON(data map[string]interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		return
	}
	fmt.Println(string(jsonData))
}

// formatBytes formats a byte count as a human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
