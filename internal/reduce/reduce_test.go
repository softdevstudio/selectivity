package reduce

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The test executable doubles as a portable predicate, without a shell or Python.
func TestOracleHelper(t *testing.T) {
	if os.Getenv("SELECTIVITY_HELPER") != "1" {
		return
	}
	args := os.Args
	idx := 0
	for i, a := range args {
		if a == "--" {
			idx = i + 1
			break
		}
	}
	if idx == 0 {
		os.Exit(125)
	}
	mode, p := args[idx], args[idx+1]
	switch mode {
	case "contains":
		b, _ := os.ReadFile(p)
		if strings.Contains(string(b), "BUG\n") {
			os.Exit(0)
		}
		os.Exit(1)
	case "pair":
		b, _ := os.ReadFile(p)
		if strings.Contains(string(b), "alpha\n") && strings.Contains(string(b), "omega\n") {
			os.Exit(0)
		}
		os.Exit(1)
	case "tree":
		a, ea := os.ReadFile(filepath.Join(p, "a.txt"))
		b, eb := os.ReadFile(filepath.Join(p, "nested", "b.txt"))
		if ea == nil && eb == nil && strings.Contains(string(a), "alpha\n") && strings.Contains(string(b), "omega\n") {
			os.Exit(0)
		}
		os.Exit(1)
	case "mutate":
		b, _ := os.ReadFile(p)
		_ = os.WriteFile(p, []byte("mutation"), 0600)
		if strings.Contains(string(b), "BUG\n") {
			os.Exit(0)
		}
		os.Exit(1)
	case "timeout":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	case "invalid":
		os.Exit(125)
	case "always":
		os.Exit(0)
	case "flaky":
		state := args[idx+2]
		b, _ := os.ReadFile(state)
		_ = os.WriteFile(state, append(b, 'x'), 0600)
		if len(b)%2 == 0 {
			os.Exit(0)
		}
		os.Exit(1)
	}
	os.Exit(125)
}
func opts(t *testing.T, mode string) Options {
	t.Helper()
	t.Setenv("SELECTIVITY_HELPER", "1")
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	return Options{Command: []string{exe, "-test.run=^TestOracleHelper$", "--", mode, "{input}"}, Timeout: 5 * time.Second, Repeats: 1, MaxTests: 1000}
}
func setup(t *testing.T, data string) (string, string) {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join(root, "source")
	if e := os.WriteFile(src, []byte(data), 0640); e != nil {
		t.Fatal(e)
	}
	return src, filepath.Join(root, "run")
}
func TestLinesAndEvidence(t *testing.T) {
	src, out := setup(t, "junk\nalpha\nother\nomega\nlast")
	s, e := Run(context.Background(), src, out, opts(t, "pair"))
	if e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(filepath.Join(out, "reduced"))
	if string(b) != "alpha\nomega\n" || !s.LocallyMinimal {
		t.Fatalf("%q %+v", b, s)
	}
	original, _ := os.ReadFile(src)
	if string(original) != "junk\nalpha\nother\nomega\nlast" {
		t.Fatal("source changed")
	}
	for _, p := range []string{"original", "summary.json", "report.html", "attempts.jsonl"} {
		if _, e := os.Stat(filepath.Join(out, p)); e != nil {
			t.Fatal(e)
		}
	}
	f, _ := os.Open(filepath.Join(out, "attempts.jsonl"))
	defer f.Close()
	dec := json.NewDecoder(f)
	for dec.More() {
		var a Attempt
		if e = dec.Decode(&a); e != nil {
			t.Fatal(e)
		}
		if len(a.Observations) != 1 {
			t.Fatal("missing observations")
		}
	}
}
func TestMutationDoesNotLeak(t *testing.T) {
	src, out := setup(t, "noise\nBUG\nmore\n")
	o := opts(t, "mutate")
	o.Repeats = 2
	s, e := Run(context.Background(), src, out, o)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(filepath.Join(out, "reduced"))
	if string(b) != "BUG\n" || s.Status != "complete" {
		t.Fatalf("%s %+v", b, s)
	}
	b, _ = os.ReadFile(src)
	if string(b) != "noise\nBUG\nmore\n" {
		t.Fatal("source mutated")
	}
}
func TestDirectoryAndLineReduction(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "source")
	_ = os.MkdirAll(filepath.Join(src, "nested"), 0700)
	_ = os.Mkdir(filepath.Join(src, "empty"), 0700)
	for n, b := range map[string]string{"a.txt": "extra\nalpha\nunused\n", "nested/b.txt": "noise\nomega\n", "junk": "unused"} {
		_ = os.WriteFile(filepath.Join(src, n), []byte(b), 0600)
	}
	out := filepath.Join(root, "run")
	o := opts(t, "tree")
	o.LinesInFiles = true
	s, e := Run(context.Background(), src, out, o)
	if e != nil {
		t.Fatal(e)
	}
	if s.ReducedFiles != 2 || !s.LocallyMinimal {
		t.Fatalf("%+v", s)
	}
	a, _ := os.ReadFile(filepath.Join(out, "reduced", "a.txt"))
	b, _ := os.ReadFile(filepath.Join(out, "reduced", "nested", "b.txt"))
	if string(a) != "alpha\n" || string(b) != "omega\n" {
		t.Fatalf("%q %q", a, b)
	}
	if _, e = os.Stat(filepath.Join(out, "original", "empty")); e != nil {
		t.Fatal("empty directory lost")
	}
}
func TestRejectBaseline(t *testing.T) {
	for _, mode := range []string{"invalid", "timeout", "flaky"} {
		t.Run(mode, func(t *testing.T) {
			src, out := setup(t, "BUG\n")
			o := opts(t, mode)
			o.Timeout = 40 * time.Millisecond
			if mode == "flaky" {
				o.Timeout = 5 * time.Second
				o.Repeats = 3
				o.Command = append(o.Command, filepath.Join(t.TempDir(), "state"))
			}
			s, e := Run(context.Background(), src, out, o)
			if e == nil || s.LocallyMinimal || s.Status != "baseline_rejected" {
				t.Fatalf("%+v %v", s, e)
			}
		})
	}
}
func TestBudgetAndCancellation(t *testing.T) {
	src, out := setup(t, "one\ntwo\nBUG\nthree\nfour\n")
	o := opts(t, "contains")
	o.MaxTests = 2
	s, e := Run(context.Background(), src, out, o)
	if e == nil || s.Status != "incomplete" || s.LocallyMinimal {
		t.Fatalf("%+v %v", s, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = Run(ctx, src, filepath.Join(filepath.Dir(out), "canceled"), opts(t, "contains"))
	if e == nil {
		t.Fatal("expected cancellation")
	}
}
func TestEmptyAndNoFinalNewline(t *testing.T) {
	for _, data := range []string{"", "one\ntwo", "one\r\ntwo\r\n"} {
		t.Run(fmt.Sprintf("size%d", len(data)), func(t *testing.T) {
			src, out := setup(t, data)
			s, e := Run(context.Background(), src, out, opts(t, "always"))
			if e != nil || s.ReducedBytes != 0 {
				t.Fatalf("%+v %v", s, e)
			}
			b, _ := os.ReadFile(filepath.Join(out, "original"))
			if string(b) != data {
				t.Fatal("line endings changed")
			}
		})
	}
}
func TestPathSafety(t *testing.T) {
	src, out := setup(t, "BUG\n")
	o := opts(t, "contains")
	_ = os.Mkdir(out, 0700)
	if _, e := Run(context.Background(), src, out, o); e == nil {
		t.Fatal("overwrote output")
	}
	dir := t.TempDir()
	if _, e := Run(context.Background(), dir, filepath.Join(dir, "inside"), o); e == nil {
		t.Fatal("accepted nested output")
	}
	if os.PathSeparator != '\\' {
		link := filepath.Join(t.TempDir(), "link")
		if e := os.Symlink(src, link); e != nil {
			t.Fatal(e)
		}
		if _, e := Load(link); e == nil {
			t.Fatal("accepted symlink")
		}
	}
	o.Command = []string{"anything"}
	if _, e := Run(context.Background(), src, filepath.Join(t.TempDir(), "out"), o); e == nil {
		t.Fatal("missing placeholder accepted")
	}
}
func TestLinesPreserveBytes(t *testing.T) {
	for _, data := range []string{"", "x", "x\n", "\n", "a\r\nb\nlast"} {
		var b strings.Builder
		for _, l := range Lines([]byte(data)) {
			b.Write(l)
		}
		if b.String() != data {
			t.Fatalf("%q", data)
		}
	}
}

func TestCanceledExecutionIsRecorded(t *testing.T) {
	src, out := setup(t, "BUG\n")
	o := opts(t, "timeout")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	s, err := Run(ctx, src, out, o)
	if err == nil || s.Tests != 1 {
		t.Fatalf("%+v %v", s, err)
	}
	b, e := os.ReadFile(filepath.Join(out, "attempts.jsonl"))
	if e != nil {
		t.Fatal(e)
	}
	var a Attempt
	if e = json.Unmarshal(b, &a); e != nil {
		t.Fatal(e)
	}
	if len(a.Observations) != 1 || a.Observations[0].Outcome != "canceled" || a.Accepted {
		t.Fatalf("%+v", a)
	}
}
func TestHashIncludesEmptyDirectories(t *testing.T) {
	a := Input{Directory: true}
	b := a
	b.Dirs = []string{"empty"}
	if a.Hash() == b.Hash() {
		t.Fatal("directory omitted from digest")
	}
}
