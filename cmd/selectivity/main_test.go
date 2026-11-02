package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCLIFlags(t *testing.T) {
	for _, tt := range []struct {
		args []string
		want int
	}{{[]string{"--version"}, 0}, {[]string{"--help"}, 0}, {nil, 2}, {[]string{"--unknown"}, 2}, {[]string{"--input", "x", "--out", "y", "--", "definitely-not-a-selectivity-program"}, 2}} {
		if got := run(tt.args); got != tt.want {
			t.Fatalf("%v: got %d want %d", tt.args, got, tt.want)
		}
	}
}

// With a real input and a real program, only the option check can refuse
// these, and it must do so before the output folder exists.
func TestCLIRefusesEmptyUnitsAndZeroJobs(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "in.txt")
	if err := os.WriteFile(input, []byte("BUG\n"), 0600); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, extra := range [][]string{{"--jobs", "0"}, {"--units", ""}, {"--jobs", "-3"}} {
		out := filepath.Join(dir, "out")
		args := append(append([]string{"--input", input, "--out", out}, extra...), "--", exe, "{input}")
		if got := run(args); got != 2 {
			t.Fatalf("%v: got %d", extra, got)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatalf("%v: output folder created", extra)
		}
	}
}
