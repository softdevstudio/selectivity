# Selectivity

Shrink a failing input while a command confirms that the behavior you care about survives. Selectivity is a single Go binary with no runtime service or third party module dependencies. It reduces lines in a file, whole files in a directory, and optionally lines inside the remaining directory files.

## Quick start

Build with Go 1.25 or newer:

```sh
go build -o bin/selectivity ./cmd/selectivity
bin/selectivity --version
bin/selectivity --input failing.csv --out reduction -- grep -q 'BAD_ROW' '{input}'
```

The output directory must be new, its parent must exist, and it must be outside the source. The command runs in a fresh temporary directory for every observation. `{input}` is replaced inside each argument with the absolute path of that observation's candidate. Commands are executed directly, without an implied shell. Use absolute paths for scripts and support files. The executable itself is resolved before changing working directory.

The predicate contract is deliberate:

| Exit | Meaning |
| --- | --- |
| 0 | The target behavior is present; a reduction may be kept |
| 1 | The target behavior is absent |
| Other exit, signal, timeout | Unresolved; never keep the reduction |

A crashing program usually needs a wrapper that maps its specific failure to exit 0. Do not treat every nonzero exit as the bug: a missing header or invalid configuration can cause an unrelated error. A wrapper should validate the candidate first, return 125 for invalid input, and distinguish the target failure using its error signature or expected exit status. See [predicate reference](docs/predicates.md).

```sh
bin/selectivity --input config-bundle --out config-reduction --lines-in-files \
  --timeout 3s --repeats 3 --max-tests 10000 -- /absolute/path/check-config '{input}'
```

All options precede `--`. Defaults are three observations per candidate, five seconds per execution, and a budget of 10,000 predicate executions for the entire run, including baseline and final checks. Budget exhaustion or interruption exits with code 2 and leaves an incomplete checkpoint. Exit 0 means the run completed with a final predicate check; inspect `locally_minimal` separately. `--help` describes the flags.

## Results

The run directory contains `original` (a snapshot), `reduced` (latest accepted input), `attempts.jsonl`, `summary.json`, and `report.html`. Open the HTML report locally; it uses no network resources. Each candidate check records its digest, removed units, observations, bounded combined command output, timing and outcome. Accepted candidates are saved as checkpoints. The source is read into memory and never opened for writing by Selectivity. Predicates may edit their disposable copies; accepted bytes are reconstructed from the snapshot.

This is a reducer, not a security sandbox. A command can access the machine and its inherited environment, reach the network or write elsewhere if its own code does so. Run only commands you trust, preferably in an isolated environment. Reports include command arguments and up to 4 KiB of output per observation; avoid secrets in both. Input must remain unchanged while its snapshot is loaded. File data are held in memory, so large inputs need enough RAM.

## What the result means

The engine removes chunks, increases granularity when necessary and finishes with single-unit deletion checks. Directory mode repeats file removal and optional per-file line reduction until neither changes. A final check repeats the predicate on the accepted result. `locally_minimal: true` means no one remaining unit was removable under these observations, and no unresolved candidate checks occurred anywhere in the run. This conservative condition may withhold a minimality claim even when an earlier unresolved candidate is no longer relevant.

Local minimality is about the selected units, not a proof of the globally smallest reproducer. A line can contain many reducible characters. Repeated checks reduce accidental acceptance; they cannot establish that a predicate will behave consistently forever. Reorderings and substitutions are outside the first version.

Line mode preserves raw bytes, CRLF and a missing final newline. It does not parse CSV, so quoted records containing newlines can be split. Validate structure in the predicate. Directory mode supports regular files, nested paths and empty directories. It retains directory structure; directories themselves are not reduction units. Symlinks and special files are rejected. File permissions are copied; directory permissions, timestamps, ownership, hardlink relationships and extended attributes are outside the input model. Unix execution uses a process group to terminate descendants after a check; Windows uses taskkill on timeout, and commands should not leave background processes behind.

Line numbers in an attempt refer to that line-reduction pass's starting file. Stages restart after a directory pass, so use the candidate digest and attempt order when correlating them. Checkpoints always contain snapshot bytes, excluding predicate side effects.

## Demonstration and verification

```sh
sh scripts/demo.sh
sh scripts/record-results.sh
```

The demo builds the binary, generates a 20,000-line file containing two necessary trigger lines, reduces it, and verifies the two-line result. It also reduces a directory down to the two necessary files and their trigger lines. It saves reports in `demo-output`. The result recorder replaces `test/results/latest` and runs vet, race tests and the demos. CI runs on Linux and macOS. See [algorithm and limits](docs/algorithm.md) for details.

## Development

The separate website repository is [photozeitgeist/selectivity.net](https://github.com/photozeitgeist/selectivity.net), including recorded browser playback of published-binary reductions.

Read [CLAUDE.md](CLAUDE.md) for the agreed plan and current handoff. Apache 2.0; see LICENSE and NOTICE. The website is a separate final stage.
