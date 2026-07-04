# supersleep - Enhanced sleep with progress tracking

[![Test](https://github.com/Harnish/supersleep/actions/workflows/test.yml/badge.svg)](https://github.com/Harnish/supersleep/actions/workflows/test.yml)

A drop-in replacement for `sleep` with real-time progress output and accurate timing. Perfect for long-running sleeps where you want to check progress without starting over.

## Features

- **Accurate timing** - Uses elapsed time tracking to prevent drift, even for very long sleeps
- **Two output modes** - Text countdown or visual progress bar
- **Script-safe interactive controls** (when run in a terminal):
  - **Any key**: Shows time remaining without exiting
  - **Ctrl-C**: Exits supersleep only — no signal is sent to a surrounding
    script/process group, so peeking at the time can't interrupt your script
  - When stdin is piped/redirected (no terminal), falls back to signal handling:
    first ctrl-c shows time remaining, second within 2s exits
- **Multiple time formats** - Support for seconds (s), minutes (m), hours (h), and days (d)
- **Fractional durations** - Decimals like `0.5` or `2.5h`
- **Summed durations** - `supersleep 1m 30s` sleeps for 90 seconds
- **Infinite sleep** - `supersleep infinity`
- **Input validation** - Errors on invalid/missing time intervals (non-zero exit)
- **Fast refresh rate** - 2-second update intervals for responsive feedback

## Installation

```bash
go build
```

## Usage

```bash
./supersleep TIME [OPTIONS]
```

### Time Format
- `30s` - 30 seconds
- `5m` - 5 minutes
- `2.5h` - 2.5 hours (decimals allowed)
- `1d` - 1 day
- Plain number - interpreted as seconds
- `infinity` - sleep forever (progress-bar mode falls back to text)
- Fused segments: `1h30m`, `2h15m30s` (any order, summed)
- Multiple values are summed: `1m 30s` = 90 seconds

### Options

- `-t` - Text mode (shows time remaining in seconds)
- `-b` - Bar mode (shows progress bar with percentage)
- `-h`, `--help` - Show help and exit
- `-v`, `--version` - Show version and exit
- (default) - Progress bar mode if no option specified

### Examples

```bash
# Sleep for 14 minutes with text countdown
./supersleep 14m -t

# Sleep for 1 hour with progress bar
./supersleep 1h -b

# Sleep for 30 seconds (default mode)
./supersleep 30s
```

### Interactive Commands

While sleeping in a terminal:
- **Any key** - Display time remaining and continue
- **Ctrl-C** - Exit supersleep (does not signal the surrounding script)

When stdin is not a terminal (piped/redirected), supersleep uses signals instead:
- **Ctrl-C** (once) - Display time remaining and continue
- **Ctrl-C** (twice within 2 seconds) - Exit immediately

## Improvements Made (v2)

- Fixed time drift issues with elapsed-time based tracking instead of fixed intervals
- Added interactive ctrl-c signal handling for displaying time remaining
- Cleaner code structure and error handling
