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
	s = Summary{Status: "incomplete", OriginalBytes: in.Bytes(), OriginalFiles: len(in.Files), Command: opts.Command, Repeats: opts.Repeats, Timeout: opts.Timeout.String(), Scope: "lines"}
	if in.Directory {
		s.Scope = "files"
		if opts.LinesInFiles {
			s.Scope = "files and per-file lines"
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
		if a.Outcome != "interesting" && a.Outcome != "uninteresting" {
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
					if err = reduceLines(&e, best, f, checkpoint); err != nil {
						return s, err
					}
				}
			}
			if best.Hash() == before {
				break
			}
		}
	} else {
		if err = reduceLines(&e, best, 0, checkpoint); err != nil {
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
func reduceLines(e *engine, base Input, f int, accept func(Input) error) error {
	lines := Lines(base.Files[f].Data)
	_, err := e.shrink(sequence(len(lines)), "lines:"+base.Files[f].Name, func(keep []int) Input {
		c := base.Clone()
		var b bytes.Buffer
		for _, i := range keep {
			b.Write(lines[i])
		}
		c.Files[f].Data = b.Bytes()
		return c
	}, func(i int) string { return fmt.Sprintf("%s:%d", base.Files[f].Name, i+1) }, accept)
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
		for lo := 0; lo < len(ids); lo += size {
			hi := lo + size
			if hi > len(ids) {
				hi = len(ids)
			}
			keep := append(append([]int(nil), ids[:lo]...), ids[hi:]...)
			removed := make([]string, 0, hi-lo)
			for _, i := range ids[lo:hi] {
				removed = append(removed, label(i))
			}
			c := build(keep)
			yes, err := e.check(c, stage, removed)
			if err != nil {
				return ids, err
			}
			if yes {
				if err = accept(c); err != nil {
					return ids, err
				}
				ids = keep
				if n > 2 {
					n--
				}
				changed = true
				break
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
	const page = `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Selectivity reduction report</title><style>body{font:17px system-ui;max-width:900px;margin:3rem auto;padding:0 1rem;background:#f5f6f8;color:#18283b}h1{font-size:2.2rem}table{border-collapse:collapse;width:100%;background:white}td,th{padding:12px;border:1px solid #ccd5df;text-align:left}code{word-break:break-all}p{line-height:1.6}</style><h1>Selectivity reduction report</h1><p>Status: {{.Status}}. {{.Error}}</p><table><tr><th>Measure</th><th>Original</th><th>Reduced</th></tr><tr><td>Bytes</td><td>{{.OriginalBytes}}</td><td>{{.ReducedBytes}}</td></tr><tr><td>Files</td><td>{{.OriginalFiles}}</td><td>{{.ReducedFiles}}</td></tr></table><p>{{.Tests}} predicate executions across {{.Attempts}} candidate checks. Unresolved checks: {{.Unresolved}}. Scope: {{.Scope}}. Locally minimal: {{.LocallyMinimal}}.</p><p>Locally minimal means no single remaining unit could be removed while satisfying the predicate under the configured repeat checks. It does not establish the globally smallest reproducer or prove future determinism. Directory structure is retained; directories themselves are not reduction units.</p><p>Repeated checks: {{.Repeats}}. Per-execution timeout: {{.Timeout}}. Elapsed: {{.DurationMS}} ms.</p><p>Command arguments: <code>{{.Command}}</code></p><p><a href="attempts.jsonl">Attempt log</a> | <a href="summary.json">JSON summary</a>. The original snapshot and latest accepted input are beside this report, named original and reduced. An incomplete run contains a checkpoint, without a final verification claim.</p></html>`
	f, err := os.OpenFile(filepath.Join(dir, "report.html"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return template.Must(template.New("report").Parse(page)).Execute(f, s)
}
