"""Recognize one particular compiler diagnostic, without executing the input."""
import sys
from pathlib import Path


def interesting(path):
    text = Path(path).read_text()
    try:
        compile(text, "candidate.py", "exec")
    except SyntaxError as error:
        return 0 if error.msg == "'break' outside loop" else 125
    return 1


if __name__ == "__main__":
    try:
        result = interesting(sys.argv[1])
    except (OSError, UnicodeError, ValueError):
        result = 125
    sys.exit(result)
