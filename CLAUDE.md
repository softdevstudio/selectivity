# Selectivity

## Project facts

Selectivity reduces an input while a user-supplied command confirms that the behavior of interest survives.
Domain: selectivity.net. Code: softdevstudio/selectivity. Site repo has not been supplied. Language: Go. License: Apache 2.0.

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

2026-10-24: Implemented line reduction, directory file reduction and optional per-file line reduction to a fixed point. Fresh predicate copies preserve the source and prevent mutations entering checkpoints. Baseline/final repeat checks, execution budget, per-process timeout, JSONL audit, JSON summary and HTML report are implemented. Exact file permissions and empty directories are modeled; unsupported metadata is documented. Checkpoints are staged before swapping.

Independent review by a separate task found interrupted-check audit loss, umask-altered permissions and omitted directory hashes. Fixed these and added regression tests; the review reran race tests and found no remaining reducer correctness issue. A second review pass checked cancellation accounting and release logic. Release verification must still check published artifacts.

Local vet, race tests and both CLI demos pass. The 20,000-line demo reduces 408,866 bytes to the two trigger lines (12 bytes). The directory demo reduces three files to two files and their required lines. Windows amd64 and macOS arm64 cross-builds pass locally; native macOS/Windows execution is delegated to CI/release smoke jobs. CI and automatic release workflows are ready; v0.1.0 publication is pending the first push and green CI. No website work has started because its repository has not been supplied.

## Next

Push the verified implementation to main; check CI and the automated v0.1.0 release, including every published asset at tag and latest plus a downloaded binary. Record release evidence and update this handoff. Website remains the final stage after a site repo is supplied.
