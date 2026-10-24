# Algorithm and guarantees

Selectivity uses complement-based delta debugging. Start with the ordered list of units, partition it into two chunks, and try deleting each chunk. Keep a deletion only after every configured observation returns interesting. If no deletion works, split more finely. Finish at singleton deletions, restarting after every acceptance. Empty candidates are tested when the last unit is removed. An empty starting input is checked at baseline and final verification.

For directory input, the outer loop reduces files, optionally reduces lines in each remaining file, and repeats until the input hash stops changing. A full unchanged loop establishes a deletion fixed point for both selected unit types. Every accepted deletion strictly reduces the number of units in that pass, so termination follows for finite input and terminating predicates. Tests can grow quadratically for difficult inputs; max-tests bounds executions, not bytes or wall-clock duration.

The initial snapshot stores file names, bytes and permissions, and the directory tree. Predicate changes to disposable candidates do not enter this model. SHA-256 digests identify the modeled input, rather than a compressed archive or the raw bytes of one file. Directory paths and file boundaries contribute to the digest.

No globally minimal claim is made. Predicate observations are finite; repeatability is a setting, not a proof. Unresolved results never justify deletion and prevent a locally_minimal claim. On incomplete runs, reduced holds the latest checkpoint and must not be treated as finally verified. Write or storage failures can leave partial artifacts; summary/report errors are returned to the caller.
