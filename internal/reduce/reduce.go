package reduce

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Summary struct {
	Status         string   `json:"status"`
	Error          string   `json:"error,omitempty"`
	OriginalBytes  int64    `json:"original_bytes"`
	ReducedBytes   int64    `json:"reduced_bytes"`
	OriginalFiles  int      `json:"original_files"`
	ReducedFiles   int      `json:"reduced_files"`
	Tests          int      `json:"predicate_executions"`
	Attempts       int      `json:"candidate_checks"`
	Unresolved     int      `json:"unresolved_checks"`
	LocallyMinimal bool     `json:"locally_minimal"`
	Scope          string   `json:"scope"`
	DurationMS     int64    `json:"duration_ms"`
	Command        []string `json:"command"`
	Repeats        int      `json:"repeats"`
	Timeout        string   `json:"timeout"`
	Units          string   `json:"units"`
	Jobs           int      `json:"jobs"`
}

// levels lists the passes for a finest unit, coarsest first.
func levels(units string) ([]string, string, bool) {
	switch units {
	case "", "lines":
		return []string{"lines"}, "lines", true
	case "tokens":
		return []string{"lines", "tokens"}, "lines and tokens", true
	case "chars":
		return []string{"lines", "tokens", "chars"}, "lines, tokens and characters", true
	}
	return nil, "", false
}

// Run reserves a new output directory. It never writes to the source path.
func Run(ctx context.Context, source, output string, opts Options) (s Summary, retErr error) {
	if len(opts.Command) == 0 || opts.Repeats < 1 || opts.Timeout <= 0 || opts.MaxTests < 2 || opts.Repeats > opts.MaxTests/2 {
		return s, fmt.Errorf("command, positive timeout/repeats and budget >= 2*repeats required")
	}
	hasInput := false
	for _, a := range opts.Command {
		if strings.Contains(a, "{input}") {
			hasInput = true
		}
	}
	if !hasInput {
		return s, fmt.Errorf("predicate arguments must contain {input}")
	}
	if opts.Units == "" {
		opts.Units = "lines"
	}
	passes, scope, ok := levels(opts.Units)
	if !ok {
		return s, fmt.Errorf("units must be lines, tokens or chars")
	}
	if opts.Jobs == 0 {
		opts.Jobs = 1
	}
	if opts.Jobs < 1 || opts.Jobs > MaxJobs {
		return s, fmt.Errorf("jobs must be from 1 to %d", MaxJobs)
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return s, err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return s, err
	}
	// Resolve parents to prevent a symlinked output parent bypassing containment.
	sp, err := filepath.EvalSymlinks(filepath.Dir(source))
	if err != nil {
		return s, err
	}
	source = filepath.Join(sp, filepath.Base(source))
	op, err := filepath.EvalSymlinks(filepath.Dir(output))
	if err != nil {
		return s, err
	}
	output = filepath.Join(op, filepath.Base(output))
	rel, err := filepath.Rel(source, output)
	if err != nil {
		return s, err
	}
	if rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return s, fmt.Errorf("output must be outside the source")
	}
	in, err := Load(source)
	if err != nil {
		return s, err
	}
	if err = os.Mkdir(output, 0700); err != nil {
		return s, fmt.Errorf("reserve output (must not exist): %w", err)
	}
	start := time.Now()
	best := in.Clone()
	s = Summary{Status: "incomplete", OriginalBytes: in.Bytes(), OriginalFiles: len(in.Files), Command: opts.Command, Repeats: opts.Repeats, Timeout: opts.Timeout.String(), Scope: scope, Units: opts.Units, Jobs: opts.Jobs}
	if in.Directory {
		s.Scope = "files"
		if opts.LinesInFiles {
			s.Scope = "files and per-file " + scope
		} else {
			// Only whole files are removed; the units setting is not used.
			s.Units = "files"
		}
	}
	e := engine{ctx: ctx, opts: opts}
	defer func() {
		s.ReducedBytes = best.Bytes()
		s.ReducedFiles = len(best.Files)
		s.Tests = e.tests
		s.Attempts = e.attempts
		s.DurationMS = time.Since(start).Milliseconds()
		if retErr != nil {
			s.Error = retErr.Error()
		}
		if err := saveReport(output, s); err != nil {
			retErr = fmt.Errorf("%v; write report: %w", retErr, err)
		}
	}()
	if err = in.Write(filepath.Join(output, "original")); err != nil {
		return s, err
	}
	if err = best.Write(filepath.Join(output, "reduced")); err != nil {
		return s, err
	}
	log, err := os.OpenFile(filepath.Join(output, "attempts.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return s, err
	}
	defer log.Close()
	e.log = func(a Attempt) error {
		// A speculative check never affected the result, so its outcome does
		// not count against the locally-minimal claim.
		if a.Outcome != "interesting" && a.Outcome != "uninteresting" && !a.Speculative {
			s.Unresolved++
		}
		if err := json.NewEncoder(log).Encode(a); err != nil {
			return err
		}
		return log.Sync()
	}
	yes, err := e.check(in, "baseline", nil)
	if err != nil {
		return s, err
	}
	if !yes {
		s.Status = "baseline_rejected"
		return s, fmt.Errorf("original does not consistently satisfy the predicate; see attempts.jsonl")
	}
	checkpoint := func(candidate Input) error {
		staging := filepath.Join(output, ".checkpoint")
		saved := filepath.Join(output, "reduced")
		backup := filepath.Join(output, ".previous")
		if err := candidate.Write(staging); err != nil {
			return err
		}
		if err := os.Rename(saved, backup); err != nil {
			return err
		}
		if err := os.Rename(staging, saved); err != nil {
			_ = os.Rename(backup, saved)
			return err
		}
		best = candidate
		return os.RemoveAll(backup)
	}
	current := func() Input { return best }
	if in.Directory {
		for {
			before := best.Hash()
			ids := sequence(len(best.Files))
			base := best.Clone()
			_, err = e.shrink(ids, "files", func(keep []int) Input {
				c := base.Clone()
				c.Files = nil
				for _, i := range keep {
					c.Files = append(c.Files, base.Files[i])
				}
				return c
			}, func(i int) string { return base.Files[i].Name }, checkpoint)
			if err != nil {
				return s, err
			}
			if opts.LinesInFiles {
				for f := 0; f < len(best.Files); f++ {
					if err = reduceText(&e, current, f, passes, checkpoint); err != nil {
						return s, err
					}
				}
			}
			if best.Hash() == before {
				break
			}
		}
	} else {
		if err = reduceText(&e, current, 0, passes, checkpoint); err != nil {
			return s, err
		}
	}
	yes, err = e.check(best, "final", nil)
	if err != nil {
		return s, err
	}
	if !yes {
		s.Status = "final_rejected"
		return s, fmt.Errorf("final predicate check failed; no verified result")
	}
	// Unknown candidate results prevent a minimality claim, even if unrelated to
	// the final singleton pass. This deliberately conservative rule is easy to audit.
	s.Status = "complete"
	s.LocallyMinimal = s.Unresolved == 0
	return s, nil
}
func sequence(n int) []int {
	a := make([]int, n)
	for i := range a {
		a[i] = i
	}
	return a
}

// reduceText runs each pass over file f, coarsest first. With more than one
// pass the round repeats until it changes nothing, so the result is 1-minimal
// at every level. A single lines pass runs once, exactly as in 0.1.
func reduceText(e *engine, current func() Input, f int, passes []string, accept func(Input) error) error {
	for {
		before := current().Hash()
		for _, pass := range passes {
			if err := reduceUnits(e, current(), f, pass, accept); err != nil {
				return err
			}
		}
		if len(passes) == 1 || current().Hash() == before {
			return nil
		}
	}
}

// Units splits data for a pass: "lines" (each line with its newline),
// "tokens" (a run of non-whitespace bytes with the whitespace after it;
// leading whitespace is its own token) or "chars" (one UTF-8 code point, or
// one byte where the data is not valid UTF-8).
func Units(pass string, b []byte) [][]byte {
	switch pass {
	case "tokens":
		return Tokens(b)
	case "chars":
		return Chars(b)
	}
	return Lines(b)
}
func reduceUnits(e *engine, base Input, f int, pass string, accept func(Input) error) error {
	units := Units(pass, base.Files[f].Data)
	name := base.Files[f].Name
	label := func(i int) string { return fmt.Sprintf("%s:%d", name, i+1) }
	if pass != "lines" {
		kind := map[string]string{"tokens": "token", "chars": "char"}[pass]
		label = func(i int) string { return fmt.Sprintf("%s:%s %d", name, kind, i+1) }
	}
	_, err := e.shrink(sequence(len(units)), pass+":"+name, func(keep []int) Input {
		c := base.Clone()
		var b bytes.Buffer
		for _, i := range keep {
			b.Write(units[i])
		}
		c.Files[f].Data = b.Bytes()
		return c
	}, label, accept)
	return err
}
func (e *engine) shrink(ids []int, stage string, build func([]int) Input, label func(int) string, accept func(Input) error) ([]int, error) {
	if len(ids) == 0 {
		return ids, nil
	}
	n := 2
	for len(ids) > 0 {
		if n > len(ids) {
			n = len(ids)
		}
		size := (len(ids) + n - 1) / n
		changed := false
		// Check the complements in windows of --jobs, in order; the earliest
		// interesting one wins, as it would in a sequential run.
		for lo := 0; lo < len(ids) && !changed; {
			var cands []Input
			var keeps [][]int
			var removals [][]string
			for len(cands) < e.opts.Jobs && lo < len(ids) {
				hi := lo + size
				if hi > len(ids) {
					hi = len(ids)
				}
				keep := append(append([]int(nil), ids[:lo]...), ids[hi:]...)
				removed := make([]string, 0, hi-lo)
				for _, i := range ids[lo:hi] {
					removed = append(removed, label(i))
				}
				cands = append(cands, build(keep))
				keeps = append(keeps, keep)
				removals = append(removals, removed)
				lo = hi
			}
			pick, err := e.checkMany(cands, stage, removals)
			if err != nil {
				return ids, err
			}
			if pick >= 0 {
				if err = accept(cands[pick]); err != nil {
					return ids, err
				}
				ids = keeps[pick]
				if n > 2 {
					n--
				}
				changed = true
			}
		}
		if changed {
			continue
		}
		if n == len(ids) {
			break
		}
		n *= 2
		if n > len(ids) {
			n = len(ids)
		}
	}
	return ids, nil
}
func saveReport(dir string, s Summary) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(dir, "summary.json"), append(data, '\n'), 0600); err != nil {
		return err
	}
	const page = `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Selectivity reduction report</title><style>body{font:17px system-ui;max-width:900px;margin:3rem auto;padding:0 1rem;background:#f5f6f8;color:#18283b}h1{font-size:2.2rem}table{border-collapse:collapse;width:100%;background:white}td,th{padding:12px;border:1px solid #ccd5df;text-align:left}code{word-break:break-all}p{line-height:1.6}</style><h1>Selectivity reduction report</h1><p>Status: {{.Status}}. {{.Error}}</p><table><tr><th>Measure</th><th>Original</th><th>Reduced</th></tr><tr><td>Bytes</td><td>{{.OriginalBytes}}</td><td>{{.ReducedBytes}}</td></tr><tr><td>Files</td><td>{{.OriginalFiles}}</td><td>{{.ReducedFiles}}</td></tr></table><p>{{.Tests}} predicate executions across {{.Attempts}} candidate checks. Unresolved checks: {{.Unresolved}}. Scope: {{.Scope}}. Locally minimal: {{.LocallyMinimal}}.</p><p>Locally minimal means no single remaining unit could be removed while satisfying the predicate under the configured repeat checks. It does not establish the globally smallest reproducer or prove future determinism. Directory structure is retained; directories themselves are not reduction units.</p><p>Finest unit: {{.Units}}. Parallel checks: {{.Jobs}}. Repeated checks: {{.Repeats}}. Per-execution timeout: {{.Timeout}}. Elapsed: {{.DurationMS}} ms.</p><p>Command arguments: <code>{{.Command}}</code></p><p><a href="attempts.jsonl">Attempt log</a> | <a href="summary.json">JSON summary</a>. The original snapshot and latest accepted input are beside this report, named original and reduced. An incomplete run contains a checkpoint, without a final verification claim.</p></html>`
	f, err := os.OpenFile(filepath.Join(dir, "report.html"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return template.Must(template.New("report").Parse(page)).Execute(f, s)
}
