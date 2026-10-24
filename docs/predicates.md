# Writing a predicate

A predicate is a program whose first responsibility is identifying the exact behavior being reduced. Exit 0 when that behavior is present, 1 when it is absent, and 125 when the candidate cannot be evaluated. Timeouts and signals are unresolved automatically.

For a crash, check the candidate's structural validity, execute the target, and confirm the expected crash signature. A parser rejecting a malformed header is not evidence that a previously observed out-of-bounds access survives. If required headers must stay, the predicate should classify a missing header as invalid.

Predicates run with inherited environment variables, no standard input, and a fresh temporary working directory. Support files outside the candidate need absolute paths. `{input}` is the candidate file or directory, never the source. Use a shell explicitly if needed, passing the candidate as a positional argument rather than interpolating it into shell source:

```sh
selectivity --input broken.txt --out result -- sh -c 'grep -q "TRIGGER" "$1"' check '{input}'
```

The default three observations must all return 0 for acceptance. Mixed outcomes are recorded as unstable; repeated unresolved outcomes remain unresolved. A predicate that always returns 0 will legitimately reduce a file to empty. Check that your predicate actually distinguishes good and bad inputs before an expensive run.
