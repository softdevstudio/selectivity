package main

import "testing"

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
