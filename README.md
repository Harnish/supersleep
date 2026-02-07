# supersleep - Enhanced sleep with progress tracking

A drop-in replacement for `sleep` with real-time progress output and accurate timing. Perfect for long-running sleeps where you want to check progress without starting over.

## Features

- **Accurate timing** - Uses elapsed time tracking to prevent drift, even for very long sleeps
- **Two output modes** - Text countdown or visual progress bar
- **Interactive ctrl-c handling**:
  - **First ctrl-c**: Shows time remaining without exiting
  - **Second ctrl-c within 2 seconds**: Gracefully exits the sleep
- **Multiple time formats** - Support for seconds (s), minutes (m), hours (h), and days (d)
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
- `2h` - 2 hours
- `1d` - 1 day
- Plain number - interpreted as seconds

### Options

- `-t` - Text mode (shows time remaining in seconds)
- `-b` - Bar mode (shows progress bar with percentage)
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

While sleeping, press:
- **Ctrl-C** (once) - Display time remaining and continue
- **Ctrl-C** (twice within 2 seconds) - Exit immediately

## Improvements Made (v2)

- Fixed time drift issues with elapsed-time based tracking instead of fixed intervals
- Added interactive ctrl-c signal handling for displaying time remaining
- Cleaner code structure and error handling
