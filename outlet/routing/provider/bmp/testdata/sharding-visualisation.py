#!/usr/bin/env python

"""Plot the results of BenchmarkRIBConcurrent.

This script reads the output of `go test -bench RIBConcurrent` on stdin and
writes three SVG graphs next to itself:

  - read_latency.svg   read latency vs writers, one panel per reader count
  - write_latency.svg  write latency vs writers, one panel per reader count
  - heatmap_ratio.svg  speedup heatmap of 1 shard vs N shards

Run the benchmark with -count > 1 so medians are meaningful, e.g.:

  make test-bench PKG=akvorado/outlet/routing/provider/bmp \\
      GOTEST_ARGS="-bench RIBConcurrent/.*,_500000_routes -count=6" GOAMD64=v3 > bench.txt
  ./sharding-visualisation.py < bench.txt
"""

import argparse
import re
import sys
from collections import defaultdict, namedtuple
from pathlib import Path

import matplotlib.colors as mcolors
import matplotlib.pyplot as plt
import numpy as np

OUTPUT_DIR = Path(__file__).resolve().parent
COLORS = ["#1f77b4", "#ff7f0e"]
MARKERS = ["o", "s"]
LINESTYLES = ["-", "--"]

# A latency measurement: the median across runs, with the min/max bounds.
Stat = namedtuple("Stat", ("median", "lo", "hi"))


def summarize(values):
    """Reduce repeated benchmark samples to a Stat (median with min/max bounds)."""
    return Stat(float(np.median(values)), float(np.min(values)), float(np.max(values)))


def parse(text):
    """Parse benchmark output into Stat records keyed by (routes, shards, writers, readers)."""
    pattern = re.compile(
        r"BenchmarkRIBConcurrent/(\d+)_shards,_(\d+)_routes,_(\d+)_writers,_(\d+)_readers-\d+\s+"
        r"\d+\s+([\d.]+)\s+ns/read(?:\s+([\d.]+)\s+ns/write)?"
    )

    samples = defaultdict(list)
    for m in pattern.finditer(text):
        shards, routes, writers, readers = int(m[1]), int(m[2]), int(m[3]), int(m[4])
        read_ns = float(m[5])
        write_ns = float(m[6]) if m[6] else None
        samples[(routes, shards, writers, readers)].append((read_ns, write_ns))

    # Go benchmarks report nanoseconds; we plot microseconds.
    stats = {}
    for key, vals in samples.items():
        reads = np.array([r for r, _ in vals]) / 1000
        writes = np.array([w for _, w in vals if w is not None]) / 1000
        stats[key] = {
            "read": summarize(reads),
            "write": summarize(writes) if len(writes) else None,
        }
    return stats


def parse_platform(text):
    """Extract the goos/goarch/cpu banner the Go benchmark prints on stdout."""
    fields = dict(re.findall(r"^(goos|goarch|cpu):\s*(.+?)\s*$", text, re.MULTILINE))
    return (
        f"{fields.get('goos', '?')}/{fields.get('goarch', '?')}\n"
        f"{fields.get('cpu', '?')}"
    )


def axes_of(values):
    """Return the sorted distinct shards, writers and readers seen in the data."""
    shards = sorted({s for s, _, _ in values})
    writers = sorted({w for _, w, _ in values})
    readers = sorted({r for _, _, r in values})
    return shards, writers, readers


def save(fig, platform, path):
    """Tag the figure with the platform and write it as a transparent SVG."""
    if fig.get_layout_engine() is None:
        fig.tight_layout()
    fig.text(
        0.995,
        0.995,
        platform,
        ha="right",
        va="top",
        fontsize=8,
        color="grey",
        style="italic",
    )
    fig.savefig(path, transparent=True, bbox_inches="tight")
    plt.close(fig)
    print(f"{path.name} saved")


def latency_figure(stats, platform, metric, routes, path):
    """Draw a latency figure (one panel per reader count) and save it."""
    shards_list, writers_list, readers_list = axes_of(stats)
    if metric == "write":
        writers_list = [w for w in writers_list if w > 0]

    fig, axes = plt.subplots(1, len(readers_list), figsize=(16, 4), squeeze=False)
    fig.suptitle(
        f"{metric.capitalize()} latency — {routes:,} routes".replace(",", " "),
        fontsize=14,
        fontweight="bold",
        y=1.02,
    )

    for ax, nr in zip(axes[0], readers_list):
        curves = []
        for idx, ns in enumerate(shards_list):
            pts = [stats[(ns, nw, nr)][metric] for nw in writers_list]
            y = [p.median for p in pts]
            yerr = [[p.median - p.lo for p in pts], [p.hi - p.median for p in pts]]
            curves.append(y)
            ax.errorbar(
                writers_list,
                y,
                yerr=yerr,
                color=COLORS[idx % len(COLORS)],
                lw=2,
                marker=MARKERS[idx % len(MARKERS)],
                linestyle=LINESTYLES[idx % len(LINESTYLES)],
                capsize=2,
                elinewidth=0.8,
                capthick=0.8,
                label=f"{ns} shard{'s' if ns > 1 else ''}",
            )
        if len(curves) >= 2:
            ax.fill_between(
                writers_list, curves[0], curves[-1], alpha=0.15, color="grey"
            )

        ax.set_title(f"{nr} reader{'s' if nr > 1 else ''}", fontsize=11)
        ax.set_xlabel("Writers")
        ax.set_xticks(writers_list)
        ax.set_ylabel("Latency (µs)" if ax is axes[0][0] else "")
        ax.grid(True, linestyle=":", alpha=0.5)
        if ax is axes[0][0]:
            ax.legend(fontsize=9)

    save(fig, platform, path)


def heatmap_figure(stats, platform, routes, path):
    """Draw the speedup heatmap (fewest shards vs most shards) and save it."""
    shards_list, writers_list, readers_list = axes_of(stats)
    lo, hi = shards_list[0], shards_list[-1]

    def ratio(a, b):
        """Median latency ratio of a over b, NaN when either is missing."""
        if a is None or b is None:
            return np.nan
        return a.median / b.median

    read_writers = writers_list
    write_writers = [w for w in writers_list if w > 0]
    fig, axes = plt.subplots(
        1,
        2,
        figsize=(10, 4),
        constrained_layout=True,
        gridspec_kw={"width_ratios": [len(read_writers), len(write_writers)]},
    )
    fig.suptitle(
        f"{lo} shard vs {hi} shards — {routes:,} routes".replace(",", " "),
        fontsize=13,
        fontweight="bold",
    )

    panels = [
        (axes[0], "read", "Read latency ratio", read_writers),
        (axes[1], "write", "Write latency ratio", write_writers),
    ]
    matrices = [
        np.array(
            [
                [
                    ratio(stats[(lo, nw, nr)][metric], stats[(hi, nw, nr)][metric])
                    for nw in wlist
                ]
                for nr in readers_list
            ]
        )
        for _, metric, _, wlist in panels
    ]

    # Log-scaled range symmetric around 1.0, shared by both panels.
    all_valid = np.concatenate([m[~np.isnan(m)] for m in matrices])
    extreme = max(all_valid.max(), 1.0 / all_valid.min(), 1.1)
    norm = mcolors.LogNorm(vmin=1.0 / extreme, vmax=extreme)

    im = None
    for (ax, _, title, wlist), matrix in zip(panels, matrices):
        im = ax.imshow(matrix, cmap=plt.cm.RdYlGn, norm=norm, aspect="auto")

        ax.set_xticks(range(len(wlist)), [str(w) for w in wlist])
        ax.set_yticks(range(len(readers_list)), [str(r) for r in readers_list])
        ax.invert_yaxis()
        ax.set_xlabel("Writers")
        if ax is axes[0]:
            ax.set_ylabel("Readers")
        ax.set_title(title, fontsize=11)

        for i in range(len(readers_list)):
            for j in range(len(wlist)):
                val = matrix[i, j]
                if not np.isnan(val):
                    ax.text(
                        j,
                        i,
                        f"{val:.2f}×",
                        ha="center",
                        va="center",
                        fontsize=9,
                        fontweight="bold",
                        color="black",
                    )

    # Set ticks for the legend.
    cb = fig.colorbar(im, ax=list(axes))
    n = int(np.ceil(np.log2(extreme)))
    ticks = [2.0**i for i in range(-n, n + 1) if 1.0 / extreme <= 2.0**i <= extreme]
    cb.set_ticks(ticks, labels=[f"{t:g}×" for t in ticks])

    save(fig, platform, path)


def main():
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument(
        "--routes",
        type=int,
        default=500000,
        help="route count to plot when the benchmark has several (default: 500000)",
    )
    args = parser.parse_args()

    text = sys.stdin.read()
    stats = parse(text)
    if not stats:
        parser.error("no BenchmarkRIBConcurrent data on stdin")

    available = sorted({routes for routes, *_ in stats})
    if len(available) == 1:
        routes = available[0]
    elif args.routes in available:
        routes = args.routes
    else:
        parser.error(f"benchmark covers routes {available}; select one with --routes")

    selected = {key[1:]: v for key, v in stats.items() if key[0] == routes}
    platform = parse_platform(text)

    latency_figure(selected, platform, "read", routes, OUTPUT_DIR / "read_latency.svg")
    latency_figure(
        selected, platform, "write", routes, OUTPUT_DIR / "write_latency.svg"
    )
    heatmap_figure(selected, platform, routes, OUTPUT_DIR / "heatmap_ratio.svg")


if __name__ == "__main__":
    main()
