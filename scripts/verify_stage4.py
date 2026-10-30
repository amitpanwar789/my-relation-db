#!/usr/bin/env python3
# This script checks the built shell using examples and a deterministic SQL workload.
# Python's SQLite runs the same supported operations as an independent test oracle.
# Row multisets and mutation counts are compared without assuming SQL result order.
# SQLite is used only for verification; the database engine never depends on it.

import argparse
from collections import Counter
import json
from pathlib import Path
import random
import sqlite3
import subprocess


def run(binary, sql):
    process = subprocess.run(
        [str(binary)], input=sql, text=True, capture_output=True, check=True,
        encoding="utf-8", timeout=30,
    )
    assert not process.stderr, process.stderr
    assert "error:" not in process.stdout, process.stdout
    decoder = json.JSONDecoder()
    text = process.stdout
    results = []
    while text.strip():
        text = text.lstrip()
        result, end = decoder.raw_decode(text)
        results.append(result)
        text = text[end:]
    return results


def literal(value):
    if value is None:
        return "NULL"
    if isinstance(value, bool):
        return "TRUE" if value else "FALSE"
    if isinstance(value, str):
        return "'" + value.replace("'", "''") + "'"
    return str(value)


def check_examples(binary, root):
    results = run(binary, (root / "examples/stage-4.sql").read_text(encoding="utf-8"))
    assert len(results) == 16
    assert results[4]["query"]["rows"] == [[{"type": "TEXT", "value": "Neha"}]]
    for position, count in [(7, 1), (9, 0), (10, 1), (12, 0), (13, 2), (14, 2)]:
        assert results[position]["affected_rows"] == count
    assert results[15]["query"]["rows"] == []
    assert len(results[15]["query"]["columns"]) == 3
    assert len(run(binary, (root / "examples/stage-3.sql").read_text(encoding="utf-8"))) == 11


def check_workload(binary):
    rng = random.Random(4302026)
    reference = sqlite3.connect(":memory:")
    commands = []
    expected = []

    def add(sql):
        cursor = reference.execute(sql)
        if sql.startswith("SELECT"):
            expected.append(("rows", Counter(cursor.fetchall())))
        elif sql.startswith(("INSERT", "UPDATE", "DELETE")):
            expected.append(("count", cursor.rowcount))
        else:
            expected.append(("create", None))
        commands.append(sql + ";")

    ids = [None, -9223372036854775808, -2, 0, 1, 2, 9223372036854775807]
    names = [None, "", "Amit", "amit", "नमस्ते", "O'Brien;"]
    flags = [None, False, True]
    conditions = [
        "id = NULL", "NULL = NULL", "active", "active IS NULL",
        "active IS NOT NULL", "name IS NULL", "name = name", "id = id",
        "TRUE", "FALSE", "NULL", "1 <= id", "id < 9223372036854775807",
        "name < 'amit'", "FALSE AND NULL", "TRUE OR NULL",
    ]

    def condition(depth=0):
        if depth >= 2 or rng.random() < .55:
            return rng.choice(conditions)
        return "(" + condition(depth + 1) + " " + rng.choice(["AND", "OR"]) + " " + condition(depth + 1) + ")"

    def insert():
        values = [rng.choice(ids), rng.choice(names), rng.choice(flags)]
        add("INSERT INTO t (id, name, active) VALUES (" + ", ".join(map(literal, values)) + ")")

    add("CREATE TABLE t (id INTEGER, name TEXT, active BOOLEAN)")
    for _ in range(20):
        insert()
    for _ in range(160):
        choice = rng.randrange(4)
        if choice == 0:
            insert()
        elif choice == 1:
            column, values = rng.choice([("id", ids), ("name", names), ("active", flags)])
            add("UPDATE t SET " + column + " = " + literal(rng.choice(values)) + " WHERE " + condition())
        elif choice == 2:
            add("DELETE FROM t WHERE " + condition())
        else:
            add("SELECT name, id, name FROM t WHERE " + condition())
        add("SELECT * FROM t")
    reference.close()

    actual = run(binary, "\n".join(commands))
    assert len(actual) == len(expected)
    counts = Counter()
    for i, (result, (kind, wanted)) in enumerate(zip(actual, expected)):
        if kind == "rows":
            rows = Counter(tuple(value["value"] for value in row) for row in result["query"]["rows"])
            assert rows == wanted, (i, commands[i], rows, wanted)
            counts["rows"] += 1
        elif kind == "count":
            assert result["affected_rows"] == wanted, (i, commands[i], result, wanted)
            counts["count"] += 1
    print(f"PASS: examples and {len(commands)} SQL commands agree with SQLite "
          f"({counts['rows']} row-result comparisons, {counts['count']} affected-row counts).")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path, help="path to the built database shell")
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    check_examples(args.binary.resolve(), root)
    check_workload(args.binary.resolve())


if __name__ == "__main__":
    main()
