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
	"unicode"

	"github.com/schollz/progressbar/v3"
)

func main() {
	bar := false
	timeleft := false
	sec := 0
	for i := range os.Args {
		istime, tempsec := IsTime(os.Args[i])
		if os.Args[i] == "-b" {
			bar = true
		} else if os.Args[i] == "-t" {
			timeleft = true
		} else if istime {
			sec = tempsec
		}
	}

	if timeleft && bar {
		fmt.Println("Only use 1 output mode")
		os.Exit(1)
	}

	const refreshRate = 2 // seconds
	totalDuration := time.Duration(sec) * time.Second
	var pbar *progressbar.ProgressBar

	if bar {
		pbar = progressbar.Default(int64(sec))
	}

	// Start time to track actual elapsed time (prevents drift)
	startTime := time.Now()

	// Signal handling for Ctrl-C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	var lastSigTime time.Time
	var sigMutex sync.Mutex
	gracefulExit := false

	go func() {
		for range sigChan {
			sigMutex.Lock()
			now := time.Now()
			elapsed := time.Since(startTime)
			remaining := totalDuration - elapsed

			if !lastSigTime.IsZero() && now.Sub(lastSigTime) < 2*time.Second {
				// Second Ctrl-C within 2 seconds - exit
				gracefulExit = true
				sigMutex.Unlock()
				fmt.Println("\nExiting...")
				os.Exit(0)
			}

			// First Ctrl-C - show time remaining
			if remaining > 0 {
				fmt.Printf("\nTime remaining: %d seconds\n", int64(remaining.Seconds()))
			}
			lastSigTime = now
			sigMutex.Unlock()
		}
	}()

	var lastUpdateTime time.Time

	if timeleft {
		fmt.Print("\033[H\033[2J")
		fmt.Printf("Time remaining: %d seconds. Refresh Rate: %d seconds\n", sec, refreshRate)
		fmt.Println("Press Ctrl-C to see time remaining, press again within 2s to exit")
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

		// Calculate next update time
		nextUpdateTime := lastUpdateTime.Add(time.Duration(refreshRate) * time.Second)
		sleepDuration := time.Until(nextUpdateTime)

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
			fmt.Printf("Time remaining: %d seconds\n", int64(remaining.Seconds()))
		} else if bar {
			// Update progress bar to match elapsed time
			pbar.Set64(int64(elapsed.Seconds()))
		}

		lastUpdateTime = time.Now()
	}

	// Final cleanup
	if timeleft {
		fmt.Print("\033[H\033[2J")
		fmt.Println("Sleep complete!")
	} else if bar {
		pbar.Set64(int64(sec))
		pbar.Finish()
	}
}

func IsTime(foo string) (bool, int) {
	durationLevel := map[string]int{
		"m": 60,
		"s": 1,
		"h": 3600,
		"d": 86400,
	}
	istime, sec := IsNumeric(foo)
	if istime {
		return true, sec
	}
	for p := range durationLevel {
		if strings.HasSuffix(foo, p) {
			newfoo := strings.TrimSuffix(foo, p)
			istime, t := IsNumeric(newfoo)
			if istime {
				return true, t * durationLevel[p]
			}
		}

	}
	return false, -1
}

func IsNumeric(s string) (bool, int) {
	if s == "" {
		return false, -1
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false, -1
		}
	}
	mynum, err := strconv.Atoi(s)
	if err != nil {
		return false, -1
	}
	return true, mynum
}
