# Selectivity

## Where the next session starts

Selectivity 0.2 Phase A (softdev-plan item 34 in photozeitgeist/jollyroger.biz, approved by YK 2026-11-02, Phase A plus upkeep) is in progress; see the next section. Phase B (JSON/JSONL and CSV-aware units) is not approved.

## In progress: Selectivity 0.2 Phase A

What and why: after line reduction the result is often one long line still full of removable text, which finer units fix; predicate runs dominate wall time, so checking several candidates at once cuts it; the release builds on Go 1.25, which is out of support, and Windows archives ship without Windows CI.

Design decisions:
- `--units=lines|tokens|chars`, default `tokens`. A token is a run of non-whitespace bytes plus the whitespace after it (spaces, tabs and newlines; leading whitespace is its own token), so a word and its separator go together and `TRIGGER\n` stays one unit. A char is one UTF-8 code point (an invalid byte is its own unit). After the line pass, the token pass and then, with `chars`, the character pass run on what is left; the levels repeat until a full round changes nothing, so the result is 1-minimal at every level used. `--units=lines` runs exactly as 0.1 did. In directory mode the finer levels apply with `--lines-in-files`, per remaining file.
- `--jobs N` (1 to 64, default 1): ddmin's candidates for one step are checked in windows of N, each in its own temp copy; after a window finishes, the earliest interesting candidate in candidate order is accepted, which is the one a sequential run would pick, so a deterministic predicate gives the same result as `--jobs 1`. Every check is logged in candidate order. Checks after the accepted one in a window are logged with `"speculative": true` (and an interesting one also `"superseded": true`); they never change the result, and their unresolved outcomes do not count against the locally-minimal claim. The budget is reserved before each execution, so it is never exceeded. Predicates must be safe to run concurrently; the README says so.
- Summary JSON gains `units` and `jobs`; the HTML report shows them.
- Upkeep: go.mod `go 1.26`, README "Go 1.26 or newer", CI and release on `go-version: stable`, a Windows CI job (vet, tests, race tests).
- Version 0.2.0.

Steps (done / remaining):
1. [ ] Upkeep: Go 1.26, stable Go in workflows, Windows CI job.
2. [ ] Token and character units with the fixed-point loop, with tests (token and char splitting, UTF-8 and invalid bytes, levels reach a fixed point, lines mode unchanged from 0.1, directory mode with --lines-in-files, budget during a fine pass, report fields).
3. [ ] `--jobs`, with tests (same result as --jobs 1 on deterministic predicates, earliest-interesting choice, speculative and superseded marks, budget never exceeded, cancellation, unresolved speculative checks don't block minimality, attempt numbering in order).
4. [ ] README, docs/algorithm.md, docs/predicates.md, demo and release smoke updated.
5. [ ] Independent review (at most two rounds), fixes, fresh test/results.
6. [ ] Selectivity 0.2.0 released by the workflow; assets checked at tag and latest; a downloaded binary run.
7. [ ] selectivity.net post with its SVG and page updates; softdevstudio.com note; softdev-plan.md item 34 updated.

Local note: this cloud session has Go 1.24 only (Go downloads are blocked); local tests run with a scratch go.mod; CI is the real check.

## Project facts

Selectivity reduces an input while a user-supplied command confirms that the behavior of interest survives.
Domain: selectivity.net. Code: softdevstudio/selectivity. Site: photozeitgeist/selectivity.net. Language: Go. License: Apache 2.0.

## Low maintenance workflow

Work on main. This file is the only plan and session handoff, shared by Claude and Codex. AGENTS.md is a one-line pointer here. README.md is the manual. docs/ contains user reference only. No ROADMAP, STATUS, RESUME or session files. Website work comes last, after a verified code release. Do not mix in other projects or discuss domain sales or value.

## Screening record

Problem: large failing inputs make debugging and bug reporting expensive. Technology: remove chunks, run a predicate on a fresh copy, keep only removals that preserve the predicate. Differentiation target: an inspectable attempt trail, conservative handling of unstable predicates, portable reports and combined directory/file reduction. These are product goals, not claims of novelty. Potential paid work: integration into private build pipelines and support for domain-specific validity constraints. No demand or trademark clearance has been established.

## Code and completion standard

One Go binary, standard library only, Apache 2.0. Tests must cover successful reduction and failures that threaten the promise, including timeouts, flaky predicates, mutation and input preservation. CI runs vet and race tests on Linux and macOS. scripts/record-results.sh replaces test/results/latest. One evidence directory per release. An independent review must run before completion; findings and the checks belong here. Release titles: Selectivity vX.Y.Z. Automated release after green CI, archives for Linux/macOS/Windows, checksums and smoke checks. Verify published assets at the tag and latest and execute a downloaded binary.

## Site standard

Use the existing Hugo Launch theme unchanged when a site repo is supplied. Content and configuration only; single H1; ASCII titles; contact info@selectivity.net. Pages: about, contact, quick start, download, reference. Unique SVG per post; featured_image frontmatter; permanent latest-release asset links. Add the project and release note to SoftDevStudio.com at website launch, following that repo's CONTENT-PLAN.md.

## Development plan

Build line reduction and directory reduction with an optional pass over each remaining file's lines. Execute commands as argument arrays, with {input} replaced by a disposable absolute candidate path. Exit 0 means interesting, 1 means uninteresting, other outcomes are unresolved. Repeat checks, timeout each execution, impose a run budget, preserve a source snapshot and save a checkpoint after each accepted removal. Produce JSONL attempts, JSON summary and a self-contained HTML report. Test and independently review; fix issues; automate and verify release. Website last.

## Where we are

2026-10-24: Selectivity 0.1.1 is released under Apache 2.0. Line reduction, directory file reduction and optional per-file line reduction run to a deletion fixed point. Fresh predicate copies preserve source bytes and prevent mutations entering checkpoints. Baseline/final repeated checks, execution budgets, timeouts, JSONL audits, JSON summaries and HTML reports are implemented. File permissions and empty directories are modeled; unsupported metadata is documented. Checkpoints are staged before swapping.

Independent review found interrupted-check audit loss, umask-altered permissions and omitted directory hashes. All were fixed with regression tests. A follow-up review reran race tests and found no remaining reducer correctness issue. Linux and macOS CI passed. Five release archives were built; native archive smoke tests passed on Linux amd64, macOS arm64 and Windows amd64. The macOS Intel and Linux ARM64 binaries are cross-built rather than natively smoke-tested.

The first 0.1.0 release published completely, but its latest-download verification step omitted the asset pattern required by gh. Recovery 0.1.1 fixed that invocation and completed the entire release workflow. No draft was left behind. All five archives at both the 0.1.1 tag and latest independently matched SHA256SUMS, and a downloaded Linux binary reproduced the two fixture reductions. A generated 20000-line, 408866-byte file became two lines and 12 bytes in 80 predicate executions. A 39-byte, three-file bundle became two files and 12 bytes in 18 executions. These are one run each with repeats=1 in a Linux amd64 container, not performance benchmarks. Full evidence is in test/results/selectivity-0.1.1-release; 0.1.0 archive checks are in its own evidence folder.

The website at photozeitgeist/selectivity.net uses the supplied Launch theme unchanged. It has the five documentation/contact pages, recorded browser playback, six posts and unique SVGs. Hugo 0.147.9 builds and 660 internal URL/asset checks pass, with one H1 per substantive page. An independent website and playback review found no mismatch against the code or released trace data. Node state checks exercised playback controls; SVGs were rasterized and inspected. Full browser rendering was unavailable locally. Site content commit: e16a3b6f60622f16e5630c7a1d1396d3b013a632.

Selectivity was added as the twenty-ninth SoftDevStudio.com project with its own note and updated plan/feed coverage. Studio commit: e41015bc7a3866a4d1d8dc8254de8966ccc65752. The user connected selectivity.net. Public HTTPS verification passed: homepage and all seven images loaded, both recorded demo runs completed at 12 bytes (80 and 18 checks), and Quick Start and Download loaded. All five archive links plus SHA256SUMS match release asset names. Fixed Hugo smart punctuation altering the inline --version command on the download page.

2026-10-24 follow-up: Added three generated use cases and real 0.1.1 evidence under examples/use-cases and the existing release evidence folder: CSV calculation disagreement, JSON Lines precision loss, and a budget-limited checkpoint followed by a fresh run. CSV reduced 2469 to 59 bytes (201 executions, 23 unresolved); JSONL 4144 to 41 bytes (36 executions); limited Python fixture 3866 to 1014 bytes (12 executions), then 1014 to 6 bytes (36 executions). Each candidate used three observations and a five-second timeout. Independent review checked audit counts, predicate behavior, exact results and all post claims, with no blocking findings. Strengthened runner assertions and reran the example bundle successfully. Engine version remains 0.1.1.

## Next

Public site and recorded playback are live and verified. Further engine features await the user's choice. Read this file first in either Claude or Codex; work on main and keep the existing template unchanged.
