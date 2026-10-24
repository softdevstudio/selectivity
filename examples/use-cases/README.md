# Generated use examples

These examples run the released Selectivity binary against controlled fixtures. They do not claim to reproduce defects in a third party product. Predicates use only Python's standard library; the reducer itself remains one Go binary.

Run from the repository root with Python 3 and an unpacked Selectivity executable:

```sh
python3 examples/use-cases/run.py --binary /absolute/path/selectivity --out /absolute/path/new-example-output
```

The output parent must exist and the output directory must be new. The script generates each fixture and runs every candidate with three observations and a five-second timeout. It saves the original inputs, CLI output and ordinary Selectivity reports. It asserts the exact reduced inputs and verifies the source digests stayed unchanged. The recorded 0.1.1 run is in test/results/selectivity-0.1.1-release/use-examples.

The CSV predicate compares a correct accumulator with a deliberately faulty one. When an earlier discount exists, the faulty code deducts a later refund twice. The generated input has 120 unrelated sales, a discount and a refund for one target order, plus a header. Headerless candidates return 125. The saved result therefore has a conservative locally_minimal=false flag even though only the header and target pair remain.

The JSON Lines predicate compares exact integer IDs with a deliberately lossy int(float(id)) consumer. One ID, 9007199254740993, changes to 9007199254740992. This is a generated precision example rather than a claim about a JSON library bug. Each candidate must contain independently valid JSON Lines records with the expected schema.

The budget example recognizes only Python's exact SyntaxError message for a break outside a loop. It compiles the candidate and never executes its code. The first run permits 12 predicate executions and is expected to exit 2 with an incomplete checkpoint. The second starts a fresh reduction from that checkpoint in a new output directory. It does not resume a saved search state. Budget results and completed results must remain distinct in any report.
