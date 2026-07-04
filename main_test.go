package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// binPath is the compiled test binary, built once in TestMain.
var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "supersleep-test")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "supersleep")
	build := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := build.CombinedOutput(); err != nil {
		panic("build failed: " + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// run executes the built binary with args and empty (non-TTY) stdin.
func run(t *testing.T, args ...string) (stdout, stderr string, exit int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Stdin = strings.NewReader("")
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exit = 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exit = ee.ExitCode()
		} else {
			t.Fatalf("run(%v): %v", args, err)
		}
	}
	return outBuf.String(), errBuf.String(), exit
}

func TestIsTime(t *testing.T) {
	cases := []struct {
		in   string
		ok   bool
		want time.Duration
	}{
		{"90", true, 90 * time.Second},
		{"30s", true, 30 * time.Second},
		{"5m", true, 5 * time.Minute},
		{"2.5h", true, 150 * time.Minute},
		{"1d", true, 24 * time.Hour},
		{"0.5", true, 500 * time.Millisecond},
		{"1h30m", true, 90 * time.Minute},
		{"1m30s", true, 90 * time.Second},
		{"2h15m30s", true, 2*time.Hour + 15*time.Minute + 30*time.Second},
		{"30s1m", true, 90 * time.Second}, // any order
		// invalid
		{"", false, 0},
		{"abc", false, 0},
		{"1m30", false, 0}, // trailing number without suffix
		{"1x", false, 0},   // unknown suffix
		{"1..2s", false, 0},
		{"-5", false, 0},
		{"m", false, 0},
	}
	for _, c := range cases {
		ok, got := IsTime(c.in)
		if ok != c.ok {
			t.Errorf("IsTime(%q) ok=%v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("IsTime(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseArgs(t *testing.T) {
	t.Run("sum", func(t *testing.T) {
		cfg, err := parseArgs([]string{"1m", "30s"})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if cfg.duration != 90*time.Second {
			t.Errorf("duration = %v, want 90s", cfg.duration)
		}
	})
	t.Run("flags", func(t *testing.T) {
		cfg, err := parseArgs([]string{"-t", "5"})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if !cfg.timeleft || cfg.bar {
			t.Errorf("cfg = %+v, want timeleft only", cfg)
		}
	})
	t.Run("infinity", func(t *testing.T) {
		cfg, err := parseArgs([]string{"infinity"})
		if err != nil || !cfg.infinite {
			t.Errorf("cfg = %+v, err = %v", cfg, err)
		}
	})
	t.Run("help short-circuits", func(t *testing.T) {
		cfg, err := parseArgs([]string{"-h"})
		if err != nil || !cfg.showHelp {
			t.Errorf("cfg = %+v, err = %v", cfg, err)
		}
	})
	t.Run("version", func(t *testing.T) {
		cfg, err := parseArgs([]string{"--version"})
		if err != nil || !cfg.showVersion {
			t.Errorf("cfg = %+v, err = %v", cfg, err)
		}
	})
	t.Run("missing operand", func(t *testing.T) {
		if _, err := parseArgs([]string{"-t"}); !errors.Is(err, errMissingOperand) {
			t.Errorf("err = %v, want errMissingOperand", err)
		}
	})
	t.Run("both modes", func(t *testing.T) {
		if _, err := parseArgs([]string{"1", "-t", "-b"}); !errors.Is(err, errBothModes) {
			t.Errorf("err = %v, want errBothModes", err)
		}
	})
	t.Run("invalid option", func(t *testing.T) {
		if _, err := parseArgs([]string{"--nope"}); err == nil || !strings.Contains(err.Error(), "invalid option") {
			t.Errorf("err = %v, want invalid option", err)
		}
	})
	t.Run("invalid interval", func(t *testing.T) {
		if _, err := parseArgs([]string{"abc"}); err == nil || !strings.Contains(err.Error(), "invalid time interval") {
			t.Errorf("err = %v, want invalid time interval", err)
		}
	})
	t.Run("bundled flags conflict", func(t *testing.T) {
		// -tb expands to -t -b, which is a mode conflict.
		if _, err := parseArgs([]string{"1", "-tb"}); !errors.Is(err, errBothModes) {
			t.Errorf("err = %v, want errBothModes", err)
		}
	})
	t.Run("bundled with version", func(t *testing.T) {
		cfg, err := parseArgs([]string{"-tv"})
		if err != nil || !cfg.showVersion {
			t.Errorf("cfg = %+v, err = %v", cfg, err)
		}
	})
	t.Run("bundled unknown char", func(t *testing.T) {
		if _, err := parseArgs([]string{"-tx", "1"}); err == nil || !strings.Contains(err.Error(), "invalid option") {
			t.Errorf("err = %v, want invalid option", err)
		}
	})
}

func TestParseFloat(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"0", 0, true},
		{"1.5", 1.5, true},
		{"90", 90, true},
		{"", 0, false},
		{"-1", 0, false},
		{"abc", 0, false},
		{"1.2.3", 0, false},
	}
	for _, c := range cases {
		got, ok := parseFloat(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseFloat(%q) = (%v,%v), want (%v,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestFloatSeconds(t *testing.T) {
	if got := floatSeconds(1.5); got != 1500*time.Millisecond {
		t.Errorf("floatSeconds(1.5) = %v, want 1.5s", got)
	}
	if got := floatSeconds(90); got != 90*time.Second {
		t.Errorf("floatSeconds(90) = %v, want 90s", got)
	}
}

func TestCLIInvalid(t *testing.T) {
	_, stderr, exit := run(t, "abc")
	if exit != 1 {
		t.Errorf("exit = %d, want 1", exit)
	}
	if !strings.Contains(stderr, "invalid time interval 'abc'") {
		t.Errorf("stderr = %q, want invalid time interval message", stderr)
	}
}

func TestCLIMissingOperand(t *testing.T) {
	_, stderr, exit := run(t)
	if exit != 1 {
		t.Errorf("exit = %d, want 1", exit)
	}
	if !strings.Contains(stderr, "missing operand") {
		t.Errorf("stderr = %q, want missing operand", stderr)
	}
}

func TestCLIInvalidOption(t *testing.T) {
	_, stderr, exit := run(t, "--nope", "1")
	if exit != 1 {
		t.Errorf("exit = %d, want 1", exit)
	}
	if !strings.Contains(stderr, "invalid option") {
		t.Errorf("stderr = %q, want invalid option", stderr)
	}
}

func TestCLIBothModes(t *testing.T) {
	_, stderr, exit := run(t, "1", "-t", "-b")
	if exit != 1 {
		t.Errorf("exit = %d, want 1", exit)
	}
	if !strings.Contains(stderr, "Only use 1 output mode") {
		t.Errorf("stderr = %q, want mode conflict", stderr)
	}
}

func TestCLIVersion(t *testing.T) {
	stdout, _, exit := run(t, "--version")
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stdout, "supersleep "+version) {
		t.Errorf("stdout = %q, want version string", stdout)
	}
}

func TestCLIHelp(t *testing.T) {
	stdout, _, exit := run(t, "--help")
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Errorf("stdout = %q, want usage", stdout)
	}
}

func TestCLIDurationSum(t *testing.T) {
	// Two args summed: 0.15 + 0.15s = 0.3s. Verify it neither exits
	// instantly nor overshoots wildly.
	start := time.Now()
	_, _, exit := run(t, "0.15", "0.15s", "-t")
	elapsed := time.Since(start)
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if elapsed < 250*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 250ms (durations should sum)", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Errorf("elapsed = %v, want < 2s (should not overshoot)", elapsed)
	}
}

func TestCLIBarInfinityFallsBackToText(t *testing.T) {
	// -b infinity has no end to fill; it should warn and use text mode.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, "-b", "infinity")
	cmd.Stdin = strings.NewReader("")
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	_ = cmd.Run() // killed by context; ignore the resulting error

	if !strings.Contains(errBuf.String(), "progress bar unavailable for infinite sleep") {
		t.Errorf("stderr = %q, want fallback notice", errBuf.String())
	}
	if !strings.Contains(outBuf.String(), "Time remaining: infinity") {
		t.Errorf("stdout = %q, want text countdown", outBuf.String())
	}
}

func TestCLIFusedCompletes(t *testing.T) {
	// Fused sub-second duration completes and prints done message.
	stdout, _, exit := run(t, "0.2s", "-t")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stdout, "Sleep complete!") {
		t.Errorf("stdout = %q, want completion message", stdout)
	}
}
