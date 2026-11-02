# Selectivity

Shrink a failing input while a command confirms that the behavior you care about survives. Selectivity is a single Go binary with no runtime service or third party module dependencies. It reduces a file by lines and, if you ask, then by words and single characters; a directory by whole files, and optionally the text inside each remaining file. It can check several candidates at once.

## Quick start

Build with Go 1.26 or newer:

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

### Finer units

`--units` sets the finest unit Selectivity removes:

| Value | Passes | Use it when |
| --- | --- | --- |
| `lines` (default) | lines only, exactly as in 0.1 | lines are meaningful on their own, or predicate runs are expensive |
| `tokens` | lines, then tokens | a long surviving line should come down to the words that matter |
| `chars` | lines, tokens, then single characters | you want the smallest text, and runs are cheap enough for many more checks |

A token is a run of non-whitespace characters together with the whitespace after it (spaces, tabs, newlines, carriage returns, vertical tabs and form feeds), so `alpha omega\n` is two tokens and a word goes with its separator. A character is one UTF-8 code point; a byte that is not valid UTF-8 counts as one character. With more than one pass, the passes repeat until a whole round removes nothing, so the result can't lose a single line, token or character (whichever levels you chose) and still satisfy the predicate. Finer passes cost more predicate runs: the last check of each pass tries every remaining unit on its own, so a result that keeps a few thousand tokens needs several thousand runs for that alone. `--max-tests` still bounds the total, and a run that hits it stops incomplete with exit 2, keeping its last accepted checkpoint. If a token or character run runs out of budget, raise `--max-tests` or go back to `--units lines`.

### Parallel checks

`--jobs N` (1 to 64, default 1) checks up to N candidates of the same step at once, each in its own temporary copy. Selectivity waits for the whole group, then keeps the earliest candidate that was interesting, the same one a one-at-a-time run would keep, so a deterministic predicate gives the same result with any `--jobs`, as long as the budget doesn't run out. Speculative checks spend budget too, so with a tight `--max-tests` a parallel run can stop earlier than a sequential one would. Predicate runs usually dominate the time, so this cuts wall time when your machine has spare cores. Checks in a group after the one that was kept are logged with `"speculative": true` (and `"superseded": true` if they were interesting too); they never change the result and an unresolved outcome among them doesn't count against `locally_minimal`. Each check in a group gets its own copy of the candidate, so memory use grows with `--jobs` times the input size. Only use `--jobs` with a predicate that is safe to run several times at once: one that writes to a fixed path, uses a fixed port or shares other state will give wrong answers.

All options precede `--`. Defaults are three observations per candidate, five seconds per execution, and a budget of 10,000 predicate executions for the entire run, including baseline and final checks. Budget exhaustion or interruption exits with code 2 and leaves an incomplete checkpoint. Exit 0 means the run completed with a final predicate check; inspect `locally_minimal` separately. `--help` describes the flags.

## Results

The run directory contains `original` (a snapshot), `reduced` (latest accepted input), `attempts.jsonl`, `summary.json`, and `report.html`. Open the HTML report locally; it uses no network resources. Each candidate check records its digest, removed units, observations, bounded combined command output, timing and outcome. Accepted candidates are saved as checkpoints. The source is read into memory and never opened for writing by Selectivity. Predicates may edit their disposable copies; accepted bytes are reconstructed from the snapshot.

This is a reducer, not a security sandbox. A command can access the machine and its inherited environment, reach the network or write elsewhere if its own code does so. Run only commands you trust, preferably in an isolated environment. Reports include command arguments and up to 4 KiB of output per observation; avoid secrets in both. Input must remain unchanged while its snapshot is loaded. File data are held in memory, so large inputs need enough RAM.

## What the result means

The engine removes chunks, increases granularity when necessary and finishes with single-unit deletion checks. With finer units it then repeats the same over tokens and characters, and starts over from lines until nothing changes. Directory mode repeats file removal and optional per-file text reduction until neither changes. A final check repeats the predicate on the accepted result. `locally_minimal: true` means no one remaining unit was removable under these observations, and no unresolved candidate checks occurred anywhere in the run. This conservative condition may withhold a minimality claim even when an earlier unresolved candidate is no longer relevant.

Local minimality is about the selected units, not a proof of the globally smallest reproducer. With `--units=lines`, a line can still contain many reducible characters. Repeated checks reduce accidental acceptance; they cannot establish that a predicate will behave consistently forever. Reorderings and substitutions are outside the first version.

Every pass preserves raw bytes, CRLF and a missing final newline in what it keeps. No pass parses CSV, JSON or any other format, so quoted CSV records containing newlines can be split and the token and character passes will happily produce invalid syntax. Validate structure in the predicate. Directory mode supports regular files, nested paths and empty directories. It retains directory structure; directories themselves are not reduction units. Symlinks and special files are rejected. File permissions are copied; directory permissions, timestamps, ownership, hardlink relationships and extended attributes are outside the input model. Unix execution uses a process group to terminate descendants after a check; Windows uses taskkill on timeout, and commands should not leave background processes behind.

The summary's `units` field records the setting (`files` for a directory without `--lines-in-files`, where it doesn't apply) and `scope` names the passes that ran, for example `lines and tokens`. Line, token and character numbers in an attempt (`input:3`, `input:token 12`, `input:char 40`) refer to that pass's starting text. Stages restart after a directory pass, so use the candidate digest and attempt order when correlating them. Checkpoints always contain snapshot bytes, excluding predicate side effects.

## Demonstration and verification

```sh
sh scripts/demo.sh
sh scripts/record-results.sh
```

The demo builds the binary, generates a 20,000-line file containing two necessary trigger lines, reduces it, and verifies the two-line result. It also reduces a directory down to the two necessary files and their trigger lines, and a single 21 KB line down to the five characters `ERR42` with `--units chars --jobs 4`. It saves reports in `demo-output`. The result recorder replaces `test/results/latest` and runs vet, race tests and the demos. CI runs the recorder on Linux and macOS and vet and the race tests on Windows. See [algorithm and limits](docs/algorithm.md) for details.

## Development

The separate website repository is [photozeitgeist/selectivity.net](https://github.com/photozeitgeist/selectivity.net), including recorded browser playback of published-binary reductions.

Read [CLAUDE.md](CLAUDE.md) for the agreed plan and current handoff. Apache 2.0; see LICENSE and NOTICE. The website is a separate final stage.
