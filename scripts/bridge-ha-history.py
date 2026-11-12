#!/usr/bin/env python3
"""Seed and inspect aged history only inside the disposable Bridge HA test lab."""

import argparse
import json
from pathlib import Path
import sqlite3
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--entity-id", required=True)
    parser.add_argument("--seed", action="store_true")
    args = parser.parse_args()
    if not Path("/config/bridge-test-auth.json").is_file():
        raise RuntimeError("This helper requires the disposable Bridge HA test lab")
    entities = {"owned": args.entity_id, "unrelated": "sun.sun"}
    result = {}
    with sqlite3.connect("/recorder/home-assistant_v2.db", timeout=10) as database:
        database.row_factory = sqlite3.Row
        if args.seed:
            sources = {}
            for name, entity_id in entities.items():
                sources[name] = database.execute(
                    "SELECT s.* FROM states s JOIN states_meta m ON s.metadata_id=m.metadata_id "
                    "WHERE m.entity_id=? AND s.state NOT LIKE 'bridge-retention-%' "
                    "ORDER BY s.state_id DESC LIMIT 1", (entity_id,)
                ).fetchone()
                if sources[name] is None:
                    raise RuntimeError("Recorder has not committed the test entities yet")
            columns = [row["name"] for row in database.execute("PRAGMA table_info(states)") if row["name"] != "state_id"]
            quoted = ",".join('"' + column.replace('"', '""') + '"' for column in columns)
            sql = "INSERT INTO states (" + quoted + ") VALUES (" + ",".join("?" for _ in columns) + ")"
            for name, source in sources.items():
                database.execute(
                    "DELETE FROM states WHERE metadata_id=? AND state LIKE 'bridge-retention-%'",
                    (source["metadata_id"],),
                )
                for label, days in (("old", 7), ("middle", 3), ("recent", 1)):
                    if name == "unrelated" and label != "old":
                        continue
                    values = dict(source)
                    values["state"] = "bridge-retention-" + label
                    values["old_state_id"] = None
                    for field in ("last_updated_ts", "last_changed_ts", "last_reported_ts"):
                        if field in values:
                            values[field] = time.time() - days * 86400
                    database.execute(sql, [values[column] for column in columns])
        for name, entity_id in entities.items():
            for label in ("old", "middle", "recent"):
                result[name + "_" + label] = database.execute(
                    "SELECT COUNT(*) FROM states s JOIN states_meta m ON s.metadata_id=m.metadata_id "
                    "WHERE m.entity_id=? AND s.state=?", (entity_id, "bridge-retention-" + label)
                ).fetchone()[0]
    print(json.dumps(result, sort_keys=True))


if __name__ == "__main__":
    main()
