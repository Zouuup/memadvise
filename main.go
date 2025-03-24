package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"
	"github.com/zouuup/memadvise/internal/advisor"
	"github.com/zouuup/memadvise/internal/inspector"
	"github.com/zouuup/memadvise/internal/output"
)

func main() {
	// Preprocess arguments to handle multiple PIDs (e.g., from command substitution)
	os.Args = preprocessArgs(os.Args)

	app := &cli.App{
		Name:  "memadvise",
		Usage: "Safely mark cold memory pages in running processes",
		Description: "A command-line utility to allow advanced users and system integrators to safely and " +
			"explicitly mark cold memory pages in running Linux processes using the process_madvise syscall",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "target",
				Aliases:  []string{"t"},
				Usage:    "Target PID or comma-separated list of PIDs",
				Required: true,
			},
			&cli.IntFlag{
				Name:    "percent",
				Aliases: []string{"p"},
				Usage:   "Percentage of eligible memory pages to reclaim",
				Value:   30,
			},
			&cli.StringFlag{
				Name:    "mode",
				Aliases: []string{"m"},
				Usage:   "Reclaim strategy: cold (lazy) or pageout (eager)",
				Value:   "cold",
			},
			&cli.BoolFlag{
				Name:    "dry-run",
				Aliases: []string{"d"},
				Usage:   "Print what would be reclaimed without performing the operation",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose logging",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:    "json",
				Aliases: []string{"j"},
				Usage:   "Output results in JSON format",
				Value:   false,
			},
			&cli.Int64Flag{
				Name:    "max-bytes",
				Aliases: []string{"b"},
				Usage:   "Maximum number of bytes to reclaim (optional cap)",
				Value:   0,
			},
		},
		Action: func(c *cli.Context) error {
			return run(c)
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	// Parse targets (PIDs)
	targetStr := c.String("target")
	targetPids, err := parsePids(targetStr)
	if err != nil {
		return fmt.Errorf("invalid target PIDs: %w", err)
	}

	// Validate mode
	mode := c.String("mode")
	if mode != "cold" && mode != "pageout" {
		return fmt.Errorf("invalid mode: %s (must be 'cold' or 'pageout')", mode)
	}

	// Initialize output based on flags
	out := output.New(c.Bool("verbose"), c.Bool("json"))

	// Process each target PID
	for _, pid := range targetPids {
		// Check if PID exists
		if !inspector.PidExists(pid) {
			out.Error(fmt.Sprintf("PID %d does not exist or is not accessible", pid))
			continue
		}

		// Create process inspector
		procInspector, err := inspector.NewProcessInspector(pid)
		if err != nil {
			out.Error(fmt.Sprintf("Failed to inspect PID %d: %v", pid, err))
			continue
		}

		// Get memory stats before advice
		beforeStats, err := procInspector.GetMemoryStats()
		if err != nil {
			out.Error(fmt.Sprintf("Failed to get memory stats for PID %d: %v", pid, err))
			continue
		}

		out.MemoryStatsBefore(pid, beforeStats)

		// Get eligible memory regions
		regions, err := procInspector.GetEligibleRegions()
		if err != nil {
			out.Error(fmt.Sprintf("Failed to get memory regions for PID %d: %v", pid, err))
			continue
		}

		// Calculate reclaim budget
		percent := c.Int("percent")
		maxBytes := c.Int64("max-bytes")
		budget := calculateBudget(beforeStats.TotalRSS, percent, maxBytes)

		// Create advisor
		adv := advisor.New(pid, regions, out)

		// Execute the advice operation
		if c.Bool("dry-run") {
			out.DryRun(pid, budget, mode, len(regions))
		} else {
			err = adv.Execute(budget, mode)
			if err != nil {
				out.Error(fmt.Sprintf("Failed to execute advice on PID %d: %v", pid, err))
				continue
			}
		}

		// Get memory stats after advice (if not dry run)
		if !c.Bool("dry-run") {
			afterStats, err := procInspector.GetMemoryStats()
			if err != nil {
				out.Error(fmt.Sprintf("Failed to get memory stats for PID %d: %v", pid, err))
				continue
			}

			out.MemoryStatsAfter(pid, afterStats, beforeStats)
		}
	}

	return nil
}

func parsePids(targetStr string) ([]int, error) {
	// Split by both commas and spaces to handle both formats
	targetStr = strings.TrimSpace(targetStr)
	var pidStrs []string

	// Check if the string contains commas
	if strings.Contains(targetStr, ",") {
		pidStrs = strings.Split(targetStr, ",")
	} else {
		// If no commas, split by spaces for command substitution output
		pidStrs = strings.Fields(targetStr)
	}

	pids := make([]int, 0, len(pidStrs))

	for _, pidStr := range pidStrs {
		pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
		if err != nil {
			return nil, fmt.Errorf("invalid PID '%s': %w", pidStr, err)
		}
		if pid <= 0 {
			return nil, fmt.Errorf("invalid PID '%d': must be positive", pid)
		}
		pids = append(pids, pid)
	}

	return pids, nil
}

// calculateBudget calculates the memory budget based on the given parameters
func calculateBudget(totalRSS int64, percent int, maxBytes int64) int64 {
	if percent <= 0 || percent > 100 {
		percent = 30 // Default to 30% if invalid
	}

	budget := totalRSS * int64(percent) / 100

	if maxBytes > 0 && budget > maxBytes {
		budget = maxBytes
	}

	return budget
}

// preprocessArgs handles the case where multiple PIDs are passed as separate arguments
// due to command substitution (e.g., `pidof stress` returning multiple PIDs)
func preprocessArgs(args []string) []string {
	if len(args) <= 2 {
		return args
	}

	processed := make([]string, 0, len(args))
	targetIndex := -1
	pids := []string{}
	inPidCollection := false

	// First pass: identify where target flag is and collect PIDs
	for i, arg := range args {
		if arg == "--target" || arg == "-t" {
			targetIndex = i
			processed = append(processed, arg)
			inPidCollection = true
			continue
		}

		// Check if it's a new flag
		if strings.HasPrefix(arg, "-") {
			inPidCollection = false
			processed = append(processed, arg)
			continue
		}

		// If we're collecting PIDs, try to parse as int
		if inPidCollection {
			_, err := strconv.Atoi(arg)
			if err == nil {
				// It's a valid PID
				pids = append(pids, arg)
			} else {
				// Not a PID, stop collecting
				inPidCollection = false
				processed = append(processed, arg)
			}
		} else {
			processed = append(processed, arg)
		}
	}

	// If we found PIDs to combine
	if targetIndex >= 0 && len(pids) > 0 {
		// Insert the combined PIDs right after the target flag
		combined := strings.Join(pids, ",")
		result := make([]string, 0, len(processed)+1)

		for i, arg := range processed {
			result = append(result, arg)
			if i == targetIndex {
				result = append(result, combined)
			}
		}

		return result
	}

	return args
}
