"""Compare exact integer IDs with a deliberately lossy float conversion."""
import json
import sys
from pathlib import Path


def interesting(path):
    exact, lossy = [], []
    for line in Path(path).read_text().splitlines():
        row = json.loads(line)
        if not isinstance(row, dict) or set(row) != {"id", "value"}:
            return 125
        if type(row["id"]) is not int or not isinstance(row["value"], str):
            return 125
        exact.append(row["id"])
        lossy.append(int(float(row["id"])))
    return 0 if exact != lossy else 1


if __name__ == "__main__":
    try:
        result = interesting(sys.argv[1])
    except (OSError, UnicodeError, ValueError, OverflowError):
        result = 125
    sys.exit(result)
