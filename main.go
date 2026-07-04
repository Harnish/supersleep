package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/term"
)

const version = "2.1.0"

const usage = `Usage: supersleep NUMBER[SUFFIX]... [OPTIONS]

Pause for the sum of the given durations, with progress output.

Suffixes:
  s  seconds (default)
  m  minutes
  h  hours
  d  days

NUMBER may be a decimal (e.g. 0.5, 2.5h). Multiple durations are summed
(e.g. '1m 30s' sleeps for 90 seconds). 'infinity' sleeps forever.

Options:
  -t          Text mode (show time remaining)
  -b          Bar mode (progress bar, default)
  -h, --help  Show this help and exit
  -v, --version
              Show version and exit
`

// config holds the parsed command-line options.
type config struct {
	bar         bool
	timeleft    bool
	infinite    bool
	duration    time.Duration
	showHelp    bool
	showVersion bool
}

var (
	errMissingOperand = errors.New("missing operand")
	errBothModes      = errors.New("Only use 1 output mode")
)

// parseArgs interprets command-line arguments (excluding argv[0]). It performs
// no I/O so it can be unit tested; help/version requests are reported via the
// config, and usage errors are returned.
func parseArgs(args []string) (config, error) {
	var cfg config
	haveTime := false

	for _, arg := range args {
		switch arg {
		case "-b":
			cfg.bar = true
			continue
		case "-t":
			cfg.timeleft = true
			continue
		case "-h", "--help":
			cfg.showHelp = true
			return cfg, nil
		case "-v", "--version":
			cfg.showVersion = true
			return cfg, nil
		case "infinity", "inf":
			cfg.infinite = true
			haveTime = true
			continue
		}

		if strings.HasPrefix(arg, "-") {
			return cfg, fmt.Errorf("invalid option '%s'", arg)
		}

		istime, d := IsTime(arg)
		if !istime {
			return cfg, fmt.Errorf("invalid time interval '%s'", arg)
		}
		cfg.duration += d
		haveTime = true
	}

	if !haveTime {
		return cfg, errMissingOperand
	}
	if cfg.timeleft && cfg.bar {
		return cfg, errBothModes
	}
	if cfg.infinite {
		cfg.duration = time.Duration(1<<63 - 1)
	}
	return cfg, nil
}

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "supersleep: %v\n", err)
		if errors.Is(err, errMissingOperand) {
			fmt.Fprintln(os.Stderr, "Try 'supersleep --help' for more information.")
		}
		os.Exit(1)
	}
	if cfg.showHelp {
		fmt.Print(usage)
		os.Exit(0)
	}
	if cfg.showVersion {
		fmt.Printf("supersleep %s\n", version)
		os.Exit(0)
	}
	runSleep(cfg)
}

// runSleep performs the sleep described by cfg, rendering progress and handling
// interactive input.
func runSleep(cfg config) {
	bar := cfg.bar
	timeleft := cfg.timeleft
	infinite := cfg.infinite
	totalDuration := cfg.duration

	const refreshRate = 2 // seconds
	sec := int(totalDuration.Seconds())
	var pbar *progressbar.ProgressBar

	if bar && !infinite {
		pbar = progressbar.Default(int64(sec))
	}

	// Start time to track actual elapsed time (prevents drift)
	startTime := time.Now()

	var sigMutex sync.Mutex
	gracefulExit := false

	// remainingStr renders the time left as a human string.
	remainingStr := func() string {
		if infinite {
			return "infinity"
		}
		remaining := totalDuration - time.Since(startTime)
		if remaining < 0 {
			remaining = 0
		}
		return fmt.Sprintf("%d seconds", int64(remaining.Seconds()))
	}

	// nl is the newline sequence. In raw terminal mode OPOST is disabled,
	// so a bare "\n" won't return the cursor to column 0 — use "\r\n".
	nl := "\n"

	// Prefer reading single keystrokes from the terminal instead of trapping
	// SIGINT. A keypress never propagates to the surrounding process group, so
	// peeking at the time remaining can't interrupt a script running supersleep.
	interactive := term.IsTerminal(int(os.Stdin.Fd()))
	if interactive {
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			interactive = false
		} else {
			nl = "\r\n"
			restore := func() { term.Restore(int(os.Stdin.Fd()), oldState) }
			defer restore()

			go func() {
				buf := make([]byte, 1)
				for {
					n, err := os.Stdin.Read(buf)
					if err != nil || n == 0 {
						return
					}
					switch buf[0] {
					case 3, 4: // Ctrl-C, Ctrl-D
						restore()
						fmt.Printf("%sExiting...%s", nl, nl)
						os.Exit(0)
					default: // any other key: show time remaining
						fmt.Printf("%sTime remaining: %s%s", nl, remainingStr(), nl)
					}
				}
			}()
		}
	}

	// Fallback for non-interactive stdin (piped/redirected): keep the old
	// signal-based behavior since there is no keyboard to read.
	if !interactive {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT)
		var lastSigTime time.Time
		go func() {
			for range sigChan {
				sigMutex.Lock()
				now := time.Now()
				if !lastSigTime.IsZero() && now.Sub(lastSigTime) < 2*time.Second {
					gracefulExit = true
					sigMutex.Unlock()
					fmt.Println("\nExiting...")
					os.Exit(0)
				}
				fmt.Printf("\nTime remaining: %s\n", remainingStr())
				lastSigTime = now
				sigMutex.Unlock()
			}
		}()
	}

	var lastUpdateTime time.Time

	if timeleft {
		fmt.Print("\033[H\033[2J")
		if infinite {
			fmt.Printf("Time remaining: infinity. Refresh Rate: %d seconds%s", refreshRate, nl)
		} else {
			fmt.Printf("Time remaining: %d seconds. Refresh Rate: %d seconds%s", sec, refreshRate, nl)
		}
		if interactive {
			fmt.Printf("Press any key to see time remaining, Ctrl-C to exit%s", nl)
		} else {
			fmt.Printf("Press Ctrl-C to see time remaining, press again within 2s to exit%s", nl)
		}
		lastUpdateTime = startTime
	}

	// Main sleep loop - uses elapsed time instead of fixed sleeps to prevent drift
	for {
		// Check if we should exit gracefully
		sigMutex.Lock()
		if gracefulExit {
			sigMutex.Unlock()
			break
		}
		sigMutex.Unlock()

		elapsed := time.Since(startTime)
		if elapsed >= totalDuration {
			break
		}

		remaining := totalDuration - elapsed

		// Calculate next update time, but never sleep past the deadline
		nextUpdateTime := lastUpdateTime.Add(time.Duration(refreshRate) * time.Second)
		sleepDuration := time.Until(nextUpdateTime)
		if sleepDuration > remaining {
			sleepDuration = remaining
		}

		if sleepDuration > 0 {
			time.Sleep(sleepDuration)
		}

		// Update display or progress bar
		elapsed = time.Since(startTime)
		if elapsed >= totalDuration {
			break
		}

		remaining = totalDuration - elapsed

		if timeleft {
			fmt.Print("\033[H\033[2J")
			if infinite {
				fmt.Printf("Time remaining: infinity%s", nl)
			} else {
				fmt.Printf("Time remaining: %d seconds%s", int64(remaining.Seconds()), nl)
			}
		} else if bar && pbar != nil {
			// Update progress bar to match elapsed time
			pbar.Set64(int64(elapsed.Seconds()))
		}

		lastUpdateTime = time.Now()
	}

	// Final cleanup
	if timeleft {
		fmt.Print("\033[H\033[2J")
		fmt.Printf("Sleep complete!%s", nl)
	} else if bar && pbar != nil {
		pbar.Set64(int64(sec))
		pbar.Finish()
	}
}

// IsTime parses a duration string. Accepts a bare number (seconds), a single
// number+suffix ("2.5h"), or fused segments ("1h30m", "2h15m30s"). Suffixes
// may appear in any order and are summed. Returns false if the string is not a
// valid non-negative duration.
func IsTime(foo string) (bool, time.Duration) {
	durationLevel := map[byte]float64{
		's': 1,
		'm': 60,
		'h': 3600,
		'd': 86400,
	}

	// Bare number = seconds
	if n, ok := parseFloat(foo); ok {
		return true, floatSeconds(n)
	}

	// Fused segments: repeated <number><suffix> pairs, e.g. "1h30m".
	var total float64
	i := 0
	for i < len(foo) {
		// Consume the numeric part.
		start := i
		for i < len(foo) && (foo[i] == '.' || (foo[i] >= '0' && foo[i] <= '9')) {
			i++
		}
		if i == start || i >= len(foo) {
			return false, 0 // missing number or missing suffix
		}
		n, ok := parseFloat(foo[start:i])
		if !ok {
			return false, 0
		}
		mult, ok := durationLevel[foo[i]]
		if !ok {
			return false, 0 // unknown suffix
		}
		total += n * mult
		i++ // skip suffix
	}
	if i == 0 {
		return false, 0
	}
	return true, floatSeconds(total)
}

func floatSeconds(sec float64) time.Duration {
	return time.Duration(sec * float64(time.Second))
}

// parseFloat accepts a non-empty, non-negative decimal number.
func parseFloat(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}
