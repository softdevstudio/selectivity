# Selectivity

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

## Next

Public site and recorded playback are live and verified. Further engine features await the user's choice. Read this file first in either Claude or Codex; work on main and keep the existing template unchanged.
