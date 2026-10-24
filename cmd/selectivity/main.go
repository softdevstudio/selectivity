package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/softdevstudio/selectivity"
	"github.com/softdevstudio/selectivity/internal/reduce"
)

func main() { os.Exit(run(os.Args[1:])) }
func run(args []string) int {
	fs := flag.NewFlagSet("selectivity", flag.ContinueOnError)
	input := fs.String("input", "", "input file or directory")
	output := fs.String("out", "", "new output directory outside input (required)")
	timeout := fs.Duration("timeout", 5*time.Second, "timeout per predicate execution")
	repeats := fs.Int("repeats", 3, "consecutive matching observations required")
	budget := fs.Int("max-tests", 10000, "maximum predicate executions for the run")
	lines := fs.Bool("lines-in-files", false, "also reduce lines inside remaining directory files")
	version := fs.Bool("version", false, "print version")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: selectivity --input PATH --out NEW_DIR [options] -- PROGRAM [ARGS containing {input}]\nPredicate exits: 0 interesting, 1 uninteresting, others unresolved. No shell is implied.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *version {
		fmt.Println("Selectivity", selectivity.Version)
		return 0
	}
	if *input == "" || *output == "" || len(fs.Args()) == 0 {
		fs.Usage()
		return 2
	}
	command := append([]string(nil), fs.Args()...)
	exe, err := exec.LookPath(command[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	command[0] = exe
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	summary, err := reduce.Run(ctx, *input, *output, reduce.Options{Command: command, Timeout: *timeout, Repeats: *repeats, MaxTests: *budget, LinesInFiles: *lines})
	if err != nil {
		fmt.Fprintln(os.Stderr, "selectivity:", err)
		return 2
	}
	fmt.Printf("%d -> %d bytes; %d predicate executions. Report: %s/report.html\n", summary.OriginalBytes, summary.ReducedBytes, summary.Tests, *output)
	return 0
}
