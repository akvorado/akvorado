#!/usr/bin/env python

"""Plot the results of BenchmarkRIBConcurrent.

Two subcommands are available:

 - single: read one benchmark output on stdin and produce three SVGs (read
   latency, write latency, heatmap ratio).
 - compare: take 2+ benchmarks as PATH=LABEL pairs and produce a single SVG
   with pairwise speedup heatmaps.

Run the benchmark with -count > 1 so medians are meaningful, e.g.:

  make test-bench PKG=akvorado/outlet/routing/provider/bmp \\
      GOTEST_ARGS="-bench RIBConcurrent/.*,_500000_routes -count=6" GOAMD64=v3 > bench.txt

Then either:

  ./sharding-visualisation.py single < bench.txt
  ./sharding-visualisation.py compare \\
      before.txt='before sharding' step1.txt='step 1' step2.txt='step 2'

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

METRICS = [("read", "Read latency"), ("write", "Write latency")]


def summarize(values):
    """Reduce repeated benchmark samples to a Stat (median with min/max bounds)."""
    return Stat(float(np.median(values)), float(np.min(values)), float(np.max(values)))


def parse(text):
    """Parse benchmark output into Stat records keyed by (routes, shards, writers, readers)."""
    pattern = re.compile(
        r"BenchmarkRIBConcurrent/(?:(\d+)_shards,_)?(\d+)_routes,_(\d+)_writers,_(\d+)_readers-\d+\s+"
        r"\d+\s+([\d.]+)\s+ns/read(?:\s+([\d.]+)\s+ns/write)?"
    )

    samples = defaultdict(list)
    for m in pattern.finditer(text):
        shards = int(m[1]) if m[1] else 1
        routes, writers, readers = int(m[2]), int(m[3]), int(m[4])
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


def select_routes(stats, requested, parser):
    """Pick a route count that exists in stats, honouring --routes when ambiguous."""
    available = sorted({routes for routes, *_ in stats})
    if len(available) == 1:
        return available[0]
    if requested in available:
        return requested
    parser.error(f"benchmark covers routes {available}; select one with --routes")


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

    # Set ticks for the legend. LogNorm auto-adds sub-decade minor ticks
    # (2, 3, … × 10^k) that show up as "6×10⁻¹" etc.; turn them off.
    cb = fig.colorbar(im, ax=list(axes))
    cb.ax.minorticks_off()
    n = int(np.ceil(np.log2(extreme)))
    ticks = [2.0**i for i in range(-n, n + 1) if 1.0 / extreme <= 2.0**i <= extreme]
    cb.set_ticks(ticks, labels=[f"{t:g}×" for t in ticks])

    save(fig, platform, path)


def comparison_figure(benchmarks, platform, routes, path):
    """Draw a 2 × C heatmap grid comparing the supplied benchmarks pairwise.

    benchmarks is a list of (label, stats, shards) tuples. The grid has one row
    per metric (read, write) and one column per ordered pair (i < j) of
    benchmarks. A cell is the speedup of benchmark j over benchmark i (i.median
    / j.median, so > 1× means j is faster).

    """
    # Pool the writer / reader axes from every benchmark.
    all_keys = set()
    for _, stats, _, _ in benchmarks:
        all_keys.update(stats.keys())
    writers = sorted({w for _, w, _ in all_keys})
    readers = sorted({r for _, _, r in all_keys})
    read_writers = writers
    write_writers = [w for w in writers if w > 0]

    # Pairwise (upper-triangle) comparisons.
    pairs = [
        (i, j) for i in range(len(benchmarks)) for j in range(i + 1, len(benchmarks))
    ]
    if not pairs:
        raise ValueError("comparison needs at least two benchmarks")

    def cell(metric, bi, vi, nw, nr):
        _, b_stats, b_shards, _ = benchmarks[bi]
        _, v_stats, v_shards, _ = benchmarks[vi]
        a = b_stats.get((b_shards, nw, nr))
        a = a.get(metric) if a else None
        b = v_stats.get((v_shards, nw, nr))
        b = b.get(metric) if b else None
        if a is None or b is None:
            return np.nan
        return a.median / b.median

    def matrix(metric, bi, vi, wlist):
        return np.array(
            [[cell(metric, bi, vi, nw, nr) for nw in wlist] for nr in readers]
        )

    matrices = {}
    for mi, (metric, _) in enumerate(METRICS):
        wlist = read_writers if metric == "read" else write_writers
        for pi, (bi, vi) in enumerate(pairs):
            matrices[(mi, pi)] = (matrix(metric, bi, vi, wlist), wlist)

    def fmt(bi):
        # Hide the shard suffix for benchmarks whose data has no sharding axis.
        label, _, shards, has_shards_info = benchmarks[bi]
        if not has_shards_info:
            return label
        return f"{label} ({shards} shard{'s' if shards > 1 else ''})"

    # With a single pair, lay the two metrics out side by side rather than
    # stacked, and move the comparison title up to the suptitle.
    single_pair = len(pairs) == 1
    if single_pair:
        rows, cols = 1, len(METRICS)
        figsize = (10, 4)
    else:
        rows, cols = len(METRICS), len(pairs)
        figsize = (max(5, 4 * len(pairs) + 1), 6)

    fig, axes = plt.subplots(
        rows,
        cols,
        figsize=figsize,
        constrained_layout=True,
        squeeze=False,
    )

    suptitle = f"Variant speedup — {routes:,} routes".replace(",", " ")
    if single_pair:
        bi, vi = pairs[0]
        suptitle += f"\n{fmt(bi)} vs {fmt(vi)}"
    fig.suptitle(suptitle, fontsize=13, fontweight="bold")

    # Log-symmetric colour scale shared by every panel.
    valid = np.concatenate(
        [m[~np.isnan(m)] for m, _ in matrices.values() if (~np.isnan(m)).any()]
    )
    extreme = max(valid.max(), 1.0 / valid.min(), 1.1)
    norm = mcolors.LogNorm(vmin=1.0 / extreme, vmax=extreme)

    def ax_for(mi, pi):
        return axes[0, mi] if single_pair else axes[mi, pi]

    im = None
    for (mi, pi), (m, wlist) in matrices.items():
        ax = ax_for(mi, pi)
        im = ax.imshow(m, cmap=plt.cm.RdYlGn, norm=norm, aspect="auto")
        ax.set_xticks(range(len(wlist)), [str(w) for w in wlist])
        ax.set_yticks(range(len(readers)), [str(r) for r in readers])
        ax.invert_yaxis()
        if single_pair:
            ax.set_title(METRICS[mi][1], fontsize=11)
            ax.set_xlabel("Writers")
            if mi == 0:
                ax.set_ylabel("Readers")
        else:
            if pi == 0:
                ax.set_ylabel(f"{METRICS[mi][1]}\nReaders")
            if mi == len(METRICS) - 1:
                ax.set_xlabel("Writers")
            if mi == 0:
                bi, vi = pairs[pi]
                ax.set_title(
                    f"{fmt(bi)} vs {fmt(vi)}",
                    fontsize=9,
                )
        for ri in range(len(readers)):
            for ci in range(len(wlist)):
                val = m[ri, ci]
                if not np.isnan(val):
                    ax.text(
                        ci,
                        ri,
                        f"{val:.2f}×",
                        ha="center",
                        va="center",
                        fontsize=8,
                        fontweight="bold",
                        color="black",
                    )

    cb = fig.colorbar(im, ax=axes.ravel().tolist())
    cb.ax.minorticks_off()
    n_ticks = int(np.ceil(np.log2(extreme)))
    ticks = [
        2.0**i
        for i in range(-n_ticks, n_ticks + 1)
        if 1.0 / extreme <= 2.0**i <= extreme
    ]
    cb.set_ticks(ticks, labels=[f"{t:g}×" for t in ticks])

    save(fig, platform, path)


def cmd_single(args, parser):
    text = sys.stdin.read()
    stats = parse(text)
    if not stats:
        parser.error("no BenchmarkRIBConcurrent data on stdin")

    routes = select_routes(stats, args.routes, parser)
    selected = {key[1:]: v for key, v in stats.items() if key[0] == routes}
    platform = parse_platform(text)

    latency_figure(selected, platform, "read", routes, OUTPUT_DIR / "read_latency.svg")
    latency_figure(
        selected, platform, "write", routes, OUTPUT_DIR / "write_latency.svg"
    )
    heatmap_figure(selected, platform, routes, OUTPUT_DIR / "heatmap_ratio.svg")


def parse_benchmark_arg(arg):
    """Parse a 'path=label' string into (Path, label)."""
    if "=" not in arg:
        raise argparse.ArgumentTypeError(f"expected PATH=LABEL, got {arg!r}")
    path_str, label = arg.split("=", 1)
    label = label.strip()
    if not label:
        raise argparse.ArgumentTypeError(f"empty label in {arg!r}")
    return Path(path_str), label


def cmd_compare(args, parser):
    if len(args.benchmarks) < 2:
        parser.error("compare requires at least two PATH=LABEL arguments")

    # Parse each input file and keep its raw text for the platform banner.
    loaded = []  # list of (label, stats, text)
    for path, label in args.benchmarks:
        text = path.read_text()
        stats = parse(text)
        if not stats:
            parser.error(f"{path}: no BenchmarkRIBConcurrent data")
        loaded.append((label, stats, text))

    # Pick a route count present in every benchmark.
    common = None
    for _, stats, _ in loaded:
        rs = {r for r, *_ in stats}
        common = rs if common is None else common & rs
    common = sorted(common or [])
    if not common:
        parser.error("no common route count across benchmarks")
    if len(common) == 1:
        routes = common[0]
    elif args.routes in common:
        routes = args.routes
    else:
        parser.error(f"benchmarks share routes {common}; select one with --routes")

    # Resolve effective shards per benchmark.
    benchmarks = (
        []
    )  # list of (label, stats_for_routes, effective_shards, has_shards_info)
    for label, stats, _ in loaded:
        filtered = {key[1:]: v for key, v in stats.items() if key[0] == routes}
        have = {s for s, _, _ in filtered}
        has_shards_info = len(have) > 1
        if args.shards in have:
            effective = args.shards
        elif not has_shards_info and 1 in have:
            # No shard axis in this benchmark (e.g. pre-sharding output): fall
            # back to its single shard value, which is 1.
            effective = 1
        else:
            parser.error(
                f"{label!r}: no data for {args.shards} shards at {routes} routes"
            )
        benchmarks.append((label, filtered, effective, has_shards_info))

    platform = parse_platform(loaded[-1][2])
    comparison_figure(benchmarks, platform, routes, OUTPUT_DIR / "comparison.svg")


def main():
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    sub = parser.add_subparsers(dest="cmd", required=True)

    sp_single = sub.add_parser(
        "single",
        help="plot one benchmark run (read on stdin)",
        description=(
            "Read one benchmark output on stdin and produce read_latency.svg, "
            "write_latency.svg and heatmap_ratio.svg next to the script."
        ),
    )
    sp_single.add_argument(
        "--routes",
        type=int,
        default=500000,
        help="route count to plot when the benchmark has several",
    )
    sp_single.set_defaults(func=cmd_single)

    sp_compare = sub.add_parser(
        "compare",
        help="compare two or more benchmark runs pairwise",
        description=(
            "Compare two or more benchmark output files and produce a "
            "single comparison.svg with one column per pairwise speedup heatmap "
            "(2 metrics × C comparisons, where C = N×(N−1)/2 for N inputs)."
        ),
    )
    sp_compare.add_argument(
        "benchmarks",
        nargs="+",
        type=parse_benchmark_arg,
        metavar="PATH=LABEL",
        help="benchmark file paired with a display label",
    )
    sp_compare.add_argument(
        "--shards",
        type=int,
        default=16,
        help="target shard count to plot (fallback to 1)",
    )
    sp_compare.add_argument(
        "--routes",
        type=int,
        default=500000,
        help="route count to plot when the benchmark has several",
    )
    sp_compare.set_defaults(func=cmd_compare)

    args = parser.parse_args()
    args.func(args, parser)


if __name__ == "__main__":
    main()
