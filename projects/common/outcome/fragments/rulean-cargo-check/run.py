"""Run `cargo check --all-targets` on every crate under crates/rulean and write a statistics report."""
import json
import os
from pathlib import Path
import subprocess

root = Path(os.environ["REPO_ROOT"])
logs = root / ".local" / "outcome" / "rulean-cargo-check"
logs.mkdir(parents=True, exist_ok=True)
crates_dir = root / "crates" / "rulean"
crates = sorted(path.name for path in crates_dir.iterdir() if (path / "Cargo.toml").is_file()) if crates_dir.is_dir() else []
rows = []
for crate in crates:
    log = logs / (crate + ".log")
    with log.open("w") as output:
        code = subprocess.run(["cargo", "check", "--all-targets", "-p", crate], cwd=root,
                              stdout=output, stderr=subprocess.STDOUT).returncode
    rows.append([crate, "passed" if code == 0 else "failed"])
    print(crate + ": exit " + str(code), flush=True)
failed = sum(row[1] == "failed" for row in rows)
report = {"format": 1, "title": "Rulean cargo check", "status": "failed" if failed else "passed",
          "counts": [{"name": "Crates", "value": len(rows)}, {"name": "Failed", "value": failed}],
          "table": {"columns": ["Crate", "Status"], "rows": rows}}
Path(os.environ["REPORT"]).write_text(json.dumps(report, indent=2) + "\n")
raise SystemExit(int(bool(failed)))
