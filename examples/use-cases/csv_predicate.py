"""Generated regression example: a refund is deducted twice after a discount."""
import csv
import io
import sys
from decimal import Decimal, InvalidOperation
from pathlib import Path


def interesting(path):
    rows = list(csv.reader(io.StringIO(Path(path).read_text(), newline="")))
    if not rows or rows[0] != ["order", "kind", "amount"]:
        return 125
    records = []
    for row in rows[1:]:
        if len(row) != 3 or not row[0] or row[1] not in {"sale", "discount", "refund"}:
            return 125
        amount = Decimal(row[2])
        if not amount.is_finite() or amount < 0:
            return 125
        records.append((row[0], row[1], amount))
    reference, buggy, discounted = {}, {}, set()
    for order, kind, amount in records:
        sign = 1 if kind == "sale" else -1
        reference[order] = reference.get(order, Decimal(0)) + sign * amount
        # This deliberately faulty importer deducts a refund twice
        # when an earlier discount exists for the same order.
        buggy[order] = buggy.get(order, Decimal(0)) + sign * amount
        if kind == "discount":
            discounted.add(order)
        if kind == "refund" and order in discounted:
            buggy[order] -= amount
    return 0 if buggy != reference else 1


if __name__ == "__main__":
    try:
        result = interesting(sys.argv[1])
    except (OSError, UnicodeError, csv.Error, InvalidOperation, ValueError):
        result = 125
    sys.exit(result)
