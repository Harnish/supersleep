package main

import (
	"fmt"
	"github.com/schollz/progressbar/v3"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
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
	split := 2
	splitsecchunks, remainder := DivideWithMod(int64(sec), int64(split))
	//if bar {
	pbar := progressbar.Default(splitsecchunks)
	//}
	if timeleft {
		fmt.Print("\033[H\033[2J")
		fmt.Println("Time remaining", ((splitsecchunks * int64(split)) + remainder), "seconds remaining. Refresh Rate", split, "seconds")
	}
	time.Sleep(time.Duration(remainder) * time.Second)
	var i int64
	for i = 0; i < splitsecchunks; i++ {
		if timeleft {
			fmt.Print("\033[H\033[2J")
			fmt.Println("Time remaining", ((splitsecchunks - i) * int64(split)), "seconds remaining. Refresh Rate", split, "seconds")
		} else if bar {
			pbar.Add(1)
		}
		time.Sleep(time.Duration(split) * time.Second)
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

func DivideWithMod(numerator, denominator int64) (int64, int64) {
	answer := numerator / denominator
	remainder := numerator % denominator
	return answer, remainder
}
