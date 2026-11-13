"""Reads results.csv and writes one speedup graph per parallel implementation."""
import csv
import sys
from collections import defaultdict

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt

rows = list(csv.DictReader(open(sys.argv[1])))
seq = {r["preset"]: float(r["time_ms"]) for r in rows if r["mode"] == "seq"}
data = defaultdict(dict)  # (mode, preset) -> {threads: speedup}
for r in rows:
    if r["mode"] != "seq":
        data[(r["mode"], r["preset"])][int(r["threads"])] = seq[r["preset"]] / float(r["time_ms"])

for mode in ("static", "steal"):
    plt.figure(figsize=(6, 4))
    for (m, preset), pts in sorted(data.items()):
        if m == mode:
            xs = sorted(pts)
            plt.plot(xs, [pts[x] for x in xs], marker="o", label=preset)
    plt.plot([2, 12], [2, 12], "k--", linewidth=0.8, label="ideal")
    plt.xlabel("threads")
    plt.ylabel("speedup over sequential")
    plt.title(f"Speedup: {mode}")
    plt.legend()
    plt.grid(alpha=0.3)
    plt.tight_layout()
    plt.savefig(f"speedup_{mode}.png", dpi=150)
