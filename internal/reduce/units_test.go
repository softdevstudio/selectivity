package reduce

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func attempts(t *testing.T, out string) []Attempt {
	t.Helper()
	f, e := os.Open(filepath.Join(out, "attempts.jsonl"))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	var all []Attempt
	dec := json.NewDecoder(f)
	for dec.More() {
		var a Attempt
		if e = dec.Decode(&a); e != nil {
			t.Fatal(e)
		}
		all = append(all, a)
	}
	return all
}
func reduced(t *testing.T, out string) string {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(out, "reduced"))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

func TestTokensAndCharsSplit(t *testing.T) {
	cases := map[string][]string{
		"":                    nil,
		"one":                 {"one"},
		"  lead two\tthree\n": {"  ", "lead ", "two\t", "three\n"},
		"a  b\n\nc":           {"a  ", "b\n\n", "c"},
		"TRIGGER\n":           {"TRIGGER\n"},
		"\n":                  {"\n"},
		"café \xff\xfe end":   {"café ", "\xff\xfe ", "end"},
	}
	for in, want := range cases {
		got := Tokens([]byte(in))
		if len(got) != len(want) {
			t.Fatalf("%q: %q", in, got)
		}
		for i := range got {
			if string(got[i]) != want[i] {
				t.Fatalf("%q: %q", in, got)
			}
		}
		if string(bytes.Join(got, nil)) != in {
			t.Fatalf("%q does not join back", in)
		}
	}
	chars := Chars([]byte("aé\xff世"))
	if len(chars) != 4 || string(chars[1]) != "é" || string(chars[2]) != "\xff" || string(chars[3]) != "世" {
		t.Fatalf("%q", chars)
	}
	if string(bytes.Join(Chars([]byte("x\xc3")), nil)) != "x\xc3" {
		t.Fatal("truncated UTF-8 lost")
	}
}

func TestTokenPassShrinksInsideALine(t *testing.T) {
	src, out := setup(t, "noise noise BUG\nmore stuff\n")
	o := opts(t, "contains")
	o.Units = "tokens"
	s, e := Run(context.Background(), src, out, o)
	if e != nil {
		t.Fatal(e)
	}
	if got := reduced(t, out); got != "BUG\n" || !s.LocallyMinimal || s.Units != "tokens" || s.Scope != "lines and tokens" {
		t.Fatalf("%q %+v", got, s)
	}
	// Lines alone keep the whole line, as 0.1 did.
	src, out = setup(t, "noise noise BUG\nmore stuff\n")
	o.Units = "lines"
	s, e = Run(context.Background(), src, out, o)
	if e != nil || reduced(t, out) != "noise noise BUG\n" || s.Scope != "lines" {
		t.Fatalf("%q %+v %v", reduced(t, out), s, e)
	}
	for _, a := range attempts(t, out) {
		if strings.HasPrefix(a.Stage, "tokens") || strings.HasPrefix(a.Stage, "chars") {
			t.Fatal("lines mode ran a finer pass", a.Stage)
		}
	}
}

func TestCharPassAndFixedPoint(t *testing.T) {
	src, out := setup(t, "first line\naaXYbb tail\nlast\n")
	o := opts(t, "substr")
	o.Units = "tokens"
	if _, e := Run(context.Background(), src, out, o); e != nil {
		t.Fatal(e)
	}
	if got := reduced(t, out); got != "aaXYbb " {
		t.Fatalf("tokens: %q", got)
	}
	src, out = setup(t, "first line\naaXYbb tail\nlast\n")
	o.Units = "chars"
	s, e := Run(context.Background(), src, out, o)
	if e != nil {
		t.Fatal(e)
	}
	if got := reduced(t, out); got != "XY" || !s.LocallyMinimal || s.Scope != "lines, tokens and characters" {
		t.Fatalf("chars: %q %+v", got, s)
	}
	// Something changed in round one, so a second round ran every pass again.
	var stages []string
	for _, a := range attempts(t, out) {
		pass, _, _ := strings.Cut(a.Stage, ":")
		if len(stages) == 0 || stages[len(stages)-1] != pass {
			stages = append(stages, pass)
		}
	}
	joined := strings.Join(stages, ",")
	if !strings.Contains(joined, "lines,tokens,chars,lines,tokens,chars") {
		t.Fatal("no confirming round:", joined)
	}
}

func TestTokensJoinWordsAcrossLines(t *testing.T) {
	src, out := setup(t, "alpha x\ny omega\nnoise\n")
	o := opts(t, "words")
	o.Units = "tokens"
	if _, e := Run(context.Background(), src, out, o); e != nil {
		t.Fatal(e)
	}
	if got := reduced(t, out); got != "alpha omega\n" {
		t.Fatalf("%q", got)
	}
}

func TestDirectoryTokensInFiles(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "in")
	os.MkdirAll(filepath.Join(src, "nested"), 0700)
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("pad pad alpha\n"), 0600)
	os.WriteFile(filepath.Join(src, "nested", "b.txt"), []byte("x omega\nz\n"), 0600)
	os.WriteFile(filepath.Join(src, "c.txt"), []byte("unused\n"), 0600)
	o := opts(t, "tree")
	o.LinesInFiles = true
	o.Units = "tokens"
	out := filepath.Join(root, "run")
	s, e := Run(context.Background(), src, out, o)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := os.ReadFile(filepath.Join(out, "reduced", "a.txt"))
	b, _ := os.ReadFile(filepath.Join(out, "reduced", "nested", "b.txt"))
	if string(a) != "alpha\n" || string(b) != "omega\n" || s.ReducedFiles != 2 || s.Scope != "files and per-file lines and tokens" {
		t.Fatalf("%q %q %+v", a, b, s)
	}
}

func TestBudgetDuringTokenPassKeepsVerifiedCheckpoint(t *testing.T) {
	src, out := setup(t, "w1 w2 w3 w4 w5 w6 BUG\nother\n")
	o := opts(t, "contains")
	o.Units = "tokens"
	o.MaxTests = 6
	s, e := Run(context.Background(), src, out, o)
	if e == nil || s.Status != "incomplete" || s.LocallyMinimal || s.Tests > 6 {
		t.Fatalf("%+v %v", s, e)
	}
	if got := reduced(t, out); !strings.Contains(got, "BUG\n") {
		t.Fatalf("checkpoint lost the bug: %q", got)
	}
}

func TestUnitsAndJobsValidation(t *testing.T) {
	src, out := setup(t, "BUG\n")
	for _, o := range []Options{{Units: "words"}, {Jobs: -1}, {Jobs: MaxJobs + 1}} {
		base := opts(t, "contains")
		if o.Units != "" {
			base.Units = o.Units
		}
		base.Jobs = o.Jobs
		if _, e := Run(context.Background(), src, out, base); e == nil {
			t.Fatal("accepted", o)
		}
	}
	if _, e := os.Stat(out); !os.IsNotExist(e) {
		t.Fatal("output reserved for invalid options")
	}
}

// With a deterministic predicate, --jobs changes how many checks run at once,
// never what is accepted.
func TestJobsGiveTheSameResult(t *testing.T) {
	data := "a\nb\nc\nd\nalpha e\nf g\nh\ni omega\nj\nk\nl\n"
	var results []string
	var accepted [][]string
	for _, jobs := range []int{1, 3, 8} {
		src, out := setup(t, data)
		o := opts(t, "words")
		o.Units = "tokens"
		o.Jobs = jobs
		s, e := Run(context.Background(), src, out, o)
		if e != nil || !s.LocallyMinimal || s.Jobs != jobs {
			t.Fatal(jobs, s, e)
		}
		results = append(results, reduced(t, out))
		var hashes []string
		for i, a := range attempts(t, out) {
			if a.Number != i+1 {
				t.Fatal("attempts out of order", a.Number, i+1)
			}
			if a.Accepted {
				hashes = append(hashes, a.Hash)
			}
			if a.Accepted && a.Speculative {
				t.Fatal("speculative attempt accepted")
			}
			if a.Superseded && !a.Speculative {
				t.Fatal("superseded without speculative")
			}
		}
		accepted = append(accepted, hashes)
	}
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] || strings.Join(accepted[i], ",") != strings.Join(accepted[0], ",") {
			t.Fatalf("jobs changed the result or path: %q vs %q", results[i], results[0])
		}
	}
	if results[0] != "alpha omega\n" {
		t.Fatalf("%q", results[0])
	}
}

func TestJobsMarkSpeculativeAndKeepMinimality(t *testing.T) {
	// Step one: removing the junk half is interesting; removing the other
	// half leaves only junk, which this predicate cannot judge (exit 125).
	// A sequential run never checks it. With two jobs it runs, is logged as
	// speculative and does not count against the minimality claim.
	src, out := setup(t, "junk1\njunk2\nBUG\nTRAP\n")
	o := opts(t, "unres")
	o.Units = "lines"
	o.Jobs = 2
	s, e := Run(context.Background(), src, out, o)
	if e != nil || reduced(t, out) != "BUG\n" {
		t.Fatal(s, e, reduced(t, out))
	}
	sawSpeculative := false
	for _, a := range attempts(t, out) {
		if a.Speculative && a.Outcome == "unresolved" {
			sawSpeculative = true
		}
	}
	if !sawSpeculative || s.Unresolved != 0 || !s.LocallyMinimal {
		t.Fatalf("speculative=%v %+v", sawSpeculative, s)
	}
	// The same input with one job never meets the unresolved candidate.
	src, out = setup(t, "junk1\njunk2\nBUG\nTRAP\n")
	o.Jobs = 1
	if s, e = Run(context.Background(), src, out, o); e != nil || !s.LocallyMinimal {
		t.Fatal(s, e)
	}
}

func TestJobsRespectBudgetAndRunInParallel(t *testing.T) {
	lines := strings.Repeat("noise\n", 15) + "BUG\n"
	src, out := setup(t, lines)
	o := opts(t, "slowbug")
	o.Units = "lines"
	o.Jobs = 8
	o.MaxTests = 9
	s, e := Run(context.Background(), src, out, o)
	if e == nil || s.Tests > 9 {
		t.Fatalf("budget exceeded or ignored: %+v %v", s, e)
	}
	// Parallel checks overlap. Compare a measured one-at-a-time run with an
	// eight-job run on the same input, rather than a fixed time, because
	// process start-up costs differ a lot between systems. Two needed lines
	// at opposite ends make most candidates fail before one succeeds, which
	// is where checking several at once pays.
	pair := "alpha\n" + strings.Repeat("noise\n", 30) + "omega\n"
	timed := func(jobs int) (time.Duration, Summary) {
		src, out := setup(t, pair)
		p := opts(t, "slowpair")
		p.Units = "lines"
		p.Jobs = jobs
		start := time.Now()
		s, e := Run(context.Background(), src, out, p)
		if e != nil || reduced(t, out) != "alpha\nomega\n" {
			t.Fatal(s, e)
		}
		return time.Since(start), s
	}
	sequential, one := timed(1)
	parallel, eight := timed(8)
	t.Logf("%d checks one at a time: %v; %d checks with 8 jobs: %v", one.Tests, sequential, eight.Tests, parallel)
	if parallel*10 > sequential*7 {
		t.Fatalf("no overlap: %d checks one at a time took %v, %d checks with 8 jobs took %v", one.Tests, sequential, eight.Tests, parallel)
	}
}

func TestJobsCancellation(t *testing.T) {
	// The full input is judged at once, so the baseline passes quickly; every
	// candidate drops a needed line and its check hangs. With four jobs the
	// first group runs in parallel when the deadline cancels the run.
	src, out := setup(t, "m1\nm2\nm3\nm4\nm5\nm6\nm7\nm8\n")
	o := opts(t, "allmarks")
	o.Units = "lines"
	o.Jobs = 4
	o.Timeout = 20 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	start := time.Now()
	s, e := Run(ctx, src, out, o)
	if e == nil || time.Since(start) > 8*time.Second || s.Status != "incomplete" {
		t.Fatal("cancellation not honoured", e, time.Since(start), s.Status)
	}
	var inGroup int
	for i, a := range attempts(t, out) {
		if a.Number != i+1 {
			t.Fatal("attempts out of order")
		}
		if a.Stage != "baseline" {
			inGroup++
			if a.Outcome != "incomplete" || a.Accepted {
				t.Fatalf("canceled check recorded as %+v", a)
			}
		}
	}
	if inGroup < 2 {
		t.Fatal("no parallel group was running at cancellation", inGroup)
	}
	if got := reduced(t, out); got != "m1\nm2\nm3\nm4\nm5\nm6\nm7\nm8\n" {
		t.Fatalf("checkpoint changed by canceled checks: %q", got)
	}
}

func TestDirectoryWithoutTextPassReportsFiles(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "in")
	os.MkdirAll(filepath.Join(src, "nested"), 0700)
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("alpha\n"), 0600)
	os.WriteFile(filepath.Join(src, "nested", "b.txt"), []byte("omega\n"), 0600)
	o := opts(t, "tree")
	o.Units = "tokens"
	s, e := Run(context.Background(), src, filepath.Join(root, "run"), o)
	if e != nil || s.Units != "files" || s.Scope != "files" {
		t.Fatal(s, e)
	}
}
