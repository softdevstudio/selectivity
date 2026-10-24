package reduce

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

var ErrBudget = errors.New("predicate execution budget exhausted")

type Options struct {
	Command      []string
	Timeout      time.Duration
	Repeats      int
	MaxTests     int
	LinesInFiles bool
}
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
	Error        string        `json:"error,omitempty"`
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
	tests    int
	attempts int
	ctx      context.Context
	log      func(Attempt) error
}

func (e *engine) check(in Input, stage string, removed []string) (interestingResult bool, retErr error) {
	a := Attempt{Number: e.attempts + 1, Stage: stage, Hash: in.Hash(), Bytes: in.Bytes(), Files: len(in.Files), Removed: removed}
	e.attempts++
	defer func() {
		if retErr != nil {
			a.Outcome = "incomplete"
			a.Error = retErr.Error()
			a.Accepted = false
		}
		if err := e.log(a); err != nil {
			interestingResult = false
			retErr = fmt.Errorf("record attempt: %w", err)
		}
	}()
	interesting := true
	first := ""
	for i := 0; i < e.opts.Repeats; i++ {
		if err := e.ctx.Err(); err != nil {
			return false, err
		}
		if e.tests >= e.opts.MaxTests {
			return false, ErrBudget
		}
		o, err := e.once(in)
		if o.Started {
			e.tests++
		}
		if o.Outcome != "" {
			a.Observations = append(a.Observations, o)
		}
		if err != nil {
			return false, err
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
	return interesting, nil
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
