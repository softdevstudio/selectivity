package reduce

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var ErrBudget = errors.New("predicate execution budget exhausted")

type Options struct {
	Command      []string
	Timeout      time.Duration
	Repeats      int
	MaxTests     int
	LinesInFiles bool
	// Units is the finest unit: "lines" (the default), "tokens" or "chars".
	Units string
	// Jobs is how many candidates of one step are checked at once (default 1).
	Jobs int
}

// MaxJobs bounds --jobs.
const MaxJobs = 64

type Observation struct {
	Started    bool   `json:"started"`
	Exit       int    `json:"exit"`
	Outcome    string `json:"outcome"`
	DurationMS int64  `json:"duration_ms"`
	Output     string `json:"output,omitempty"`
}
type Attempt struct {
	Number       int           `json:"number"`
	Stage        string        `json:"stage"`
	Hash         string        `json:"sha256"`
	Bytes        int64         `json:"bytes"`
	Files        int           `json:"files"`
	Removed      []string      `json:"removed,omitempty"`
	Observations []Observation `json:"observations"`
	Outcome      string        `json:"outcome"`
	Accepted     bool          `json:"accepted"`
	// Speculative marks a check that ran in the same --jobs window after the
	// candidate that was accepted, or after one that failed. A sequential run
	// would not have made it, and its outcome never affects the result.
	Speculative bool `json:"speculative,omitempty"`
	// Superseded marks a speculative check that was itself interesting.
	Superseded bool   `json:"superseded,omitempty"`
	Error      string `json:"error,omitempty"`
}
type limitedBuffer struct{ data []byte }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if left := 4096 - len(b.data); left > 0 {
		if len(p) > left {
			p = p[:left]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}

type engine struct {
	opts     Options
	mu       sync.Mutex // guards tests while checks run in parallel
	tests    int
	attempts int
	ctx      context.Context
	log      func(Attempt) error
}

// reserve claims one predicate execution from the budget before it starts,
// so parallel checks can never exceed it.
func (e *engine) reserve() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.tests >= e.opts.MaxTests {
		return false
	}
	e.tests++
	return true
}
func (e *engine) unreserve() {
	e.mu.Lock()
	e.tests--
	e.mu.Unlock()
}

// record numbers and logs one attempt. Attempts are always recorded from one
// goroutine, in candidate order.
func (e *engine) record(a Attempt, failure error) error {
	e.attempts++
	a.Number = e.attempts
	if failure != nil {
		a.Outcome = "incomplete"
		a.Error = failure.Error()
		a.Accepted = false
	}
	if err := e.log(a); err != nil {
		return fmt.Errorf("record attempt: %w", err)
	}
	return nil
}

// check evaluates and records one candidate.
func (e *engine) check(in Input, stage string, removed []string) (bool, error) {
	a, yes, err := e.evaluate(in, stage, removed)
	if lerr := e.record(a, err); lerr != nil {
		return false, lerr
	}
	if err != nil {
		return false, err
	}
	return yes, nil
}

// checkMany evaluates candidates concurrently and returns the index of the
// first interesting one in candidate order, or -1. That is the candidate a
// sequential run would accept. Every candidate is recorded in order; those
// after the decisive one are marked speculative. The first error in order,
// if it comes before any interesting candidate, is returned.
func (e *engine) checkMany(cands []Input, stage string, removed [][]string) (int, error) {
	if len(cands) == 1 {
		yes, err := e.check(cands[0], stage, removed[0])
		if err != nil || !yes {
			return -1, err
		}
		return 0, nil
	}
	type result struct {
		a   Attempt
		yes bool
		err error
	}
	results := make([]result, len(cands))
	var wg sync.WaitGroup
	for i := range cands {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a, yes, err := e.evaluate(cands[i], stage, removed[i])
			results[i] = result{a, yes, err}
		}(i)
	}
	wg.Wait()
	decisive, accepted := len(cands), -1
	var failure error
	for i, r := range results {
		if r.err != nil {
			decisive, failure = i, r.err
			break
		}
		if r.yes {
			decisive, accepted = i, i
			break
		}
	}
	for i, r := range results {
		a := r.a
		if i > decisive {
			a.Speculative = true
			a.Superseded = r.yes && r.err == nil
			a.Accepted = false
		}
		if err := e.record(a, r.err); err != nil {
			return -1, err
		}
	}
	return accepted, failure
}

// evaluate runs the configured repeats on one candidate without recording it.
func (e *engine) evaluate(in Input, stage string, removed []string) (Attempt, bool, error) {
	a := Attempt{Stage: stage, Hash: in.Hash(), Bytes: in.Bytes(), Files: len(in.Files), Removed: removed}
	interesting := true
	first := ""
	for i := 0; i < e.opts.Repeats; i++ {
		if err := e.ctx.Err(); err != nil {
			return a, false, err
		}
		if !e.reserve() {
			return a, false, ErrBudget
		}
		o, err := e.once(in)
		if !o.Started {
			e.unreserve()
		}
		if o.Outcome != "" {
			a.Observations = append(a.Observations, o)
		}
		if err != nil {
			return a, false, err
		}
		if i == 0 {
			first = o.Outcome
		}
		if o.Outcome != first {
			a.Outcome = "unstable"
		}
		if o.Outcome != "interesting" {
			interesting = false
		}
	}
	if a.Outcome == "" {
		a.Outcome = first
	}
	a.Accepted = interesting && stage != "baseline" && stage != "final"
	return a, interesting, nil
}
func (e *engine) once(in Input) (Observation, error) {
	temp, err := os.MkdirTemp("", "selectivity-trial-")
	if err != nil {
		return Observation{}, err
	}
	defer os.RemoveAll(temp)
	p := temp + string(os.PathSeparator) + "input"
	if err = in.Write(p); err != nil {
		return Observation{}, err
	}
	args := make([]string, len(e.opts.Command))
	for i, a := range e.opts.Command {
		args[i] = strings.ReplaceAll(a, "{input}", p)
	}
	ctx, cancel := context.WithTimeout(e.ctx, e.opts.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = temp
	cmd.WaitDelay = 250 * time.Millisecond
	prepare(cmd)
	var b limitedBuffer
	cmd.Stdout = &b
	cmd.Stderr = &b
	start := time.Now()
	err = cmd.Run()
	cleanup(cmd)
	o := Observation{Started: cmd.Process != nil, Exit: -1, DurationMS: time.Since(start).Milliseconds(), Output: string(b.data)}
	if e.ctx.Err() != nil {
		o.Outcome = "canceled"
		return o, e.ctx.Err()
	}
	if ctx.Err() != nil {
		o.Outcome = "timeout"
		return o, nil
	}
	if err == nil {
		o.Exit = 0
		o.Outcome = "interesting"
		return o, nil
	}
	var ex *exec.ExitError
	if errors.As(err, &ex) {
		o.Exit = ex.ExitCode()
		o.Outcome = "unresolved"
		if o.Exit == 1 {
			o.Outcome = "uninteresting"
		}
		return o, nil
	}
	if errors.Is(err, exec.ErrWaitDelay) {
		o.Outcome = "unresolved"
		return o, nil
	}
	o.Outcome = "start_error"
	return o, fmt.Errorf("start predicate: %w", err)
}
