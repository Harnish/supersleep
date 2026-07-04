package main

import (
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

func main() {
	bar := false
	timeleft := false
	haveTime := false
	infinite := false
	var totalDuration time.Duration

	for _, arg := range os.Args[1:] {
		switch arg {
		case "-b":
			bar = true
			continue
		case "-t":
			timeleft = true
			continue
		case "-h", "--help":
			fmt.Print(usage)
			os.Exit(0)
		case "-v", "--version":
			fmt.Printf("supersleep %s\n", version)
			os.Exit(0)
		case "infinity", "inf":
			infinite = true
			haveTime = true
			continue
		}

		if strings.HasPrefix(arg, "-") {
			fmt.Fprintf(os.Stderr, "supersleep: invalid option '%s'\n", arg)
			os.Exit(1)
		}

		istime, d := IsTime(arg)
		if !istime {
			fmt.Fprintf(os.Stderr, "supersleep: invalid time interval '%s'\n", arg)
			os.Exit(1)
		}
		totalDuration += d
		haveTime = true
	}

	if !haveTime {
		fmt.Fprintln(os.Stderr, "supersleep: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'supersleep --help' for more information.")
		os.Exit(1)
	}

	if timeleft && bar {
		fmt.Fprintln(os.Stderr, "Only use 1 output mode")
		os.Exit(1)
	}

	if infinite {
		totalDuration = time.Duration(1<<63 - 1)
	}

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

// IsTime parses a duration like "30s", "5m", "2.5h", or a bare number
// (interpreted as seconds). Returns false if the string is not a valid
// non-negative duration.
func IsTime(foo string) (bool, time.Duration) {
	durationLevel := map[string]float64{
		"s": 1,
		"m": 60,
		"h": 3600,
		"d": 86400,
	}

	// Bare number = seconds
	if n, ok := parseFloat(foo); ok {
		return true, floatSeconds(n)
	}

	for p, mult := range durationLevel {
		if strings.HasSuffix(foo, p) {
			if n, ok := parseFloat(strings.TrimSuffix(foo, p)); ok {
				return true, floatSeconds(n * mult)
			}
		}
	}
	return false, 0
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
