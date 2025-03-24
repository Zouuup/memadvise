# memadvise

memadvise is a CLI tool that leverages the obscure but powerful process_madvise() syscall to proactively reclaim cold memory from specific processes-without killing them or waiting for the kernel to panic under pressure.


## 🧾 Overview

`memadvise` is a focused command-line utility that allows power users and system integrators to selectively reclaim anonymous memory pages from running Linux processes using the **little-known `process_madvise()` syscall** (introduced in Linux 5.10). Despite being a kernel-native mechanism for advisory memory management, this syscall remains largely undocumented and underutilized in userspace tooling.

Unlike traditional memory reclaim mechanisms (e.g. `kswapd`, OOM killer, or swap) that operate reactively and globally, `memadvise` allows **fine-grained, per-process reclaim** under explicit user control — with zero disruption to the process itself. It allows you to tell the kernel:  
> “This memory is safe to deprioritize or evict — do it on my terms.”

By targeting **anonymous, private, writable memory** (such as unused heap), `memadvise` can help reclaim memory from backgrounded processes, idle VMs, long-lived batch jobs, or bloated desktop applications — all without forcing the application to cooperate or exit.

Whether you're building a smarter virtualization platform, optimizing node memory usage, or just trying to keep your desktop responsive without relying on aggressive overcommit behavior, `memadvise` gives you a clean, controlled interface to a feature the Linux kernel already supports — but never made easy to use. Until now.

## Requirements

- Linux 5.10+ (for process_madvise syscall support)
- Go 1.20+

## Installation

```bash
go install github.com/zouuup/memadvise
```

### From Source

```bash
git clone https://github.com/zouuup/memadvise.git
cd memadvise
go build -o memadvise
```

### Binary Releases

Download prebuilt binaries from the [Releases](https://github.com/zouuup/memadvise/releases) page.

## Usage

```
NAME:
   memadvise - Safely mark cold memory pages in running processes

USAGE:
   memadvise [global options]

DESCRIPTION:
   A command-line utility to allow advanced users and system integrators to safely and
   explicitly mark cold memory pages in running Linux processes using the process_madvise syscall

GLOBAL OPTIONS:
   --target value, -t value    Target PID or comma-separated list of PIDs
   --percent value, -p value   Percentage of eligible memory pages to reclaim (default: 30)
   --mode value, -m value      Reclaim strategy: cold (lazy) or pageout (eager) (default: "cold")
   --dry-run, -d               Print what would be reclaimed without performing the operation (default: false)
   --verbose, -v               Enable verbose logging (default: false)
   --json, -j                  Output results in JSON format (default: false)
   --max-bytes value, -b value Maximum number of bytes to reclaim (optional cap) (default: 0)
   --help, -h                  show help
```

## Examples

Mark 30% of anonymous memory pages as cold in a Chrome process:

```bash
memadvise --target $(pidof chrome) --percent 30
```

Eagerly page out memory from a specific process:

```bash
memadvise --target 1234 --mode pageout
```

Dry run with verbose output:

```bash
memadvise --target 9923 --percent 20 --dry-run --verbose
```

Multiple targets with JSON output:

```bash
memadvise --target 9923,9924 --percent 20 --json
```

## Reclaim Modes

- `cold` (default): Marks memory as not recently used, allowing the kernel to reclaim it under memory pressure (MADV_COLD)
- `pageout`: Actively reclaims memory immediately, writing dirty pages to swap if available (MADV_PAGEOUT)

## Security Considerations

- Requires CAP_SYS_NICE or ptrace-equivalent permissions to target arbitrary processes
- Uses pidfd to validate PID liveness and prevent TOCTOU race conditions
- Validates address ranges against memory map permissions and protection flags
- Will not affect shared memory, mapped devices, JIT memory, or stack regions

## How It Works

1. Reads /proc/PID/maps to identify eligible anonymous private writable memory regions
2. Calculates reclaim budget based on specified percentage or max bytes
3. Creates page-aligned iovecs for eligible regions
4. Applies process_madvise syscall with selected mode
5. Reports memory usage before and after the operation

## License

MIT
