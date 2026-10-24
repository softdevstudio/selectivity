"""Run documented generated examples against an explicit Selectivity binary."""
import argparse
import csv
import hashlib
import json
import platform
import subprocess
import sys
from pathlib import Path


def run(binary, args, output):
    result = subprocess.run([str(binary), *args], text=True, capture_output=True)
    (output / "cli.txt").write_text(result.stdout + result.stderr)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    args = parser.parse_args()
    binary = args.binary.resolve(strict=True)
    root = args.out.resolve()
    root.mkdir(mode=0o700, parents=False, exist_ok=False)
    source = root / "source"
    source.mkdir()
    predicates = Path(__file__).resolve().parent
    with (source / "orders.csv").open("w", newline="") as stream:
        writer = csv.writer(stream, lineterminator="\n")
        writer.writerow(["order", "kind", "amount"])
        for i in range(120):
            if i == 31:
                writer.writerow(["target", "discount", "5.00"])
            if i == 89:
                writer.writerow(["target", "refund", "20.00"])
            writer.writerow([f"order-{i}", "sale", "30.00"])
    (source / "records.jsonl").write_text("".join(
        json.dumps({"id": 9007199254740993 if i == 73 else i, "value": f"row-{i}"}, separators=(",", ":")) + "\n"
        for i in range(150)
    ))
    (source / "program.py").write_text("".join(
        "break\n" if i == 180 else f"value_{i} = {i}\n" for i in range(256)
    ))
    hashes = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in source.iterdir()}
    version = subprocess.check_output([str(binary), "--version"], text=True).strip()
    (root / "environment.json").write_text(json.dumps({
        "binary": version, "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
        "machine": platform.platform(), "python": sys.version, "repeats": 3,
        "sample_count": 1, "timeout": "5s", "original_sha256": hashes,
    }, indent=2) + "\n")
    cases = [
        ("csv", source / "orders.csv", "csv_predicate.py", []),
        ("jsonl", source / "records.jsonl", "jsonl_predicate.py", []),
        ("budget", source / "program.py", "syntax_predicate.py", ["--max-tests", "12"]),
        ("continued", root / "budget" / "reduced", "syntax_predicate.py", []),
    ]
    summaries = {}
    for name, input_path, predicate, extra in cases:
        output = root / name
        result = run(binary, ["--input", str(input_path), "--out", str(output),
                              "--repeats", "3", "--timeout", "5s", *extra,
                              "--", sys.executable, str(predicates / predicate), "{input}"], output)
        expected = 2 if name == "budget" else 0
        assert result.returncode == expected, (name, result.returncode, result.stderr)
        summary = json.loads((output / "summary.json").read_text())
        summaries[name] = summary
        if name == "budget":
            assert summary["status"] == "incomplete" and not summary["locally_minimal"]
            assert "budget" in summary["error"]
            assert summary["predicate_executions"] == 12
            assert len((output / "reduced").read_text().splitlines()) == 64
            assert summary["reduced_bytes"] == 1014
        else:
            assert summary["status"] == "complete"
        print(name, json.dumps(summary))
    rows = list(csv.reader((root / "csv/reduced").read_text().splitlines()))
    assert rows == [["order", "kind", "amount"], ["target", "discount", "5.00"], ["target", "refund", "20.00"]], rows
    assert not summaries["csv"]["locally_minimal"] and summaries["csv"]["unresolved_checks"] > 0
    lines = (root / "jsonl/reduced").read_text().splitlines()
    assert len(lines) == 1 and json.loads(lines[0]) == {"id": 9007199254740993, "value": "row-73"}
    assert (root / "jsonl/reduced").stat().st_size == 41
    assert (root / "continued/reduced").read_bytes() == b"break\n"
    for name in ["csv", "jsonl", "budget", "continued"]:
        output = root / name
        assert (output / "original").read_bytes() == cases[[c[0] for c in cases].index(name)][1].read_bytes()
        for file in ["report.html", "summary.json", "attempts.jsonl"]:
            assert (output / file).is_file()
    assert hashes == {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in source.iterdir()}
    (root / "checks.txt").write_text("All generated examples matched their expected results. Source digests unchanged.\n")


if __name__ == "__main__":
    main()
