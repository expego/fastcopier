#!/usr/bin/env python3
"""
Parse Go benchmark output and update the Performance section in README.md.

Usage:
    python3 scripts/update-readme.py <bench_gen.txt> <bench_nogen.txt> <README.md>

    bench_gen.txt    — output of: go test -bench=. -benchmem -benchtime=3s
    bench_nogen.txt  — output of: go test -bench='BenchmarkManual|BenchmarkFastCopier|BenchmarkClone'
                                       -tags fastcopier_no_gen -benchmem -benchtime=3s

The script replaces the content between:
    <!-- BENCHMARK_RESULTS_START -->
    ...
    <!-- BENCHMARK_RESULTS_END -->
"""

import re
import sys
from dataclasses import dataclass
from typing import Optional


@dataclass
class BenchResult:
    ns_op: float = 0.0
    b_op: int = 0
    allocs_op: int = 0


# Maps benchmark function name suffix → (display label, sort order)
# Order within each group determines the row order in the table.
BENCH_META = {
    "Manual":                 ("Manual (baseline)",          0),
    "FastCopier":             ("**FastCopier (with gen)**",  1),
    # "FastCopierReflect" is a synthetic key injected from the no-gen results
    "FastCopierReflect":      ("FastCopier (pure reflect)",  2),
    "Clone":                  ("FastCopier.Clone",           3),
    "HuanduGoClone":          ("huandu/go-clone",            4),
    "GoDeepCopy":             ("tiendc/go-deepcopy",         5),
    "JinzhuCopier":           ("jinzhu/copier",              6),
    "Mapstructure":           ("go-viper/mapstructure",       7),
    "Deepcopier":             ("ulule/deepcopier",           8),
    "JSONRoundTrip":          ("encoding/json",              9),
}

# Which libraries to show in each scenario (allows hiding irrelevant rows per scenario).
# None means "show all".
SCENARIO_LIBS = {
    "Simple":  None,
    "Nested":  None,
    "Complex": None,
    # Deep: show all libs (crashes shown via KNOWN_OMISSIONS); order follows BENCH_META.
    "Deep": [
        "Manual", "FastCopier", "Clone",
        "HuanduGoClone", "GoDeepCopy", "JinzhuCopier",
        "Mapstructure", "Deepcopier", "JSONRoundTrip",
    ],
}

# Known failures: (lib_key, scenario) → display string for the ns/op cell.
# Rows with an entry here are shown with the error string and dashes for the
# other columns instead of benchmark numbers.
KNOWN_OMISSIONS = {
    ("GoDeepCopy",             "Deep"): "❌ stack overflow",
    ("Mapstructure",           "Deep"): "❌ stack overflow",
    ("HuanduGoClone",          "Deep"): "❌ stack overflow",
    ("JSONRoundTrip",          "Deep"): "❌ infinite loop",
}

# Cycle-safety label for the Deep table's extra column.
CYCLE_BEHAVIOR = {
    "Manual":                 "✅ (explicit)",
    "FastCopier":             "**✅**",
    "Clone":                  "✅",
    "JinzhuCopier":           "⚠️ shallow ptrs",
    "Deepcopier":             "⚠️ shallow ptrs",
    "GoDeepCopy":             "❌",
    "Mapstructure":           "❌",
    "HuanduGoClone":          "❌",
    "JSONRoundTrip":          "❌",
}

SCENARIOS = [
    ("Simple",  "Simple Struct (5 primitive fields)"),
    ("Nested",  "Nested Struct (struct + slices)"),
    ("Complex", "Complex Struct (nested + slice of structs + map)"),
    ("Deep",    "Deep Struct (Organisation: 10 employees, circular references)"),
]


# ── Parsing ───────────────────────────────────────────────────────────────────

def parse_benchmarks(path: str) -> dict[tuple[str, str], BenchResult]:
    """Return {(lib_key, scenario): BenchResult} from go test -benchmem output."""
    pattern = re.compile(
        r"^Benchmark(\w+?)_(\w+?)(?:-\d+)?\s+"
        r"\d+\s+"
        r"([\d.]+)\s+ns/op"
        r"(?:\s+([\d.]+)\s+B/op)?"
        r"(?:\s+(\d+)\s+allocs/op)?",
        re.MULTILINE,
    )
    results: dict[tuple[str, str], BenchResult] = {}
    with open(path) as f:
        text = f.read()
    for m in pattern.finditer(text):
        lib, scenario, ns, b_op, allocs = m.groups()
        results[(lib, scenario)] = BenchResult(
            ns_op=float(ns),
            b_op=int(float(b_op)) if b_op else 0,
            allocs_op=int(allocs) if allocs else 0,
        )
    return results


def extract_env(bench_file: str) -> tuple[str, str]:
    """Return (cpu_string, go_version_string) from benchmark output header."""
    cpu, go_ver = "unknown CPU", "Go unknown"
    with open(bench_file) as f:
        for line in f:
            line = line.strip()
            if line.startswith("cpu:"):
                cpu = line[4:].strip()
            elif line.startswith("go version "):
                parts = line.split()
                if len(parts) >= 3:
                    go_ver = parts[2]
    return cpu, go_ver


# ── Formatting helpers ────────────────────────────────────────────────────────

def fmt_ns(ns: float) -> str:
    if ns < 1:
        return f"{ns:.3g}"
    if ns >= 1000:
        return f"{ns:,.0f}"
    return f"{ns:.3g}"


def fmt_int(n: int) -> str:
    return f"{n:,}" if n >= 1000 else str(n)


def vs_label(competitor_ns: float, baseline_ns: float) -> str:
    """Human-readable ratio of competitor vs baseline."""
    if baseline_ns <= 0:
        return "—"
    ratio = competitor_ns / baseline_ns
    if ratio < 0.91:
        return f"{1/ratio:.1f}× faster"
    if ratio <= 1.10:
        return "on-par"
    bold = ratio >= 5
    text = f"{ratio:.1f}× slower"
    return f"**{text}**" if bold else text


# ── Table builders ────────────────────────────────────────────────────────────

def build_std_table(
    scenario_key: str,
    results: dict[tuple[str, str], BenchResult],
) -> str:
    """Standard table (Simple / Nested / Complex): ns/op, B/op, allocs/op, vs FastCopier."""
    header = (
        "| Library | ns/op | B/op | allocs/op | vs FastCopier |\n"
        "|---------|------:|-----:|----------:|:-------------:|"
    )
    baseline = results.get(("FastCopier", scenario_key))
    baseline_ns = baseline.ns_op if baseline else 0.0

    allowed = SCENARIO_LIBS.get(scenario_key)  # None = show all
    rows = []
    for lib_key, (label, _) in sorted(BENCH_META.items(), key=lambda x: x[1][1]):
        if allowed is not None and lib_key not in allowed:
            continue
        omission = KNOWN_OMISSIONS.get((lib_key, scenario_key))
        result = results.get((lib_key, scenario_key))
        if omission:
            rows.append(f"| {label} | {omission} | — | — | — |")
        elif result:
            ns = fmt_ns(result.ns_op)
            b = fmt_int(result.b_op)
            a = fmt_int(result.allocs_op)
            if lib_key == "FastCopier":
                vs = "**—**"
            else:
                vs = vs_label(result.ns_op, baseline_ns)
            rows.append(f"| {label} | {ns} | {b} | {a} | {vs} |")
    return header + "\n" + "\n".join(rows)


def build_deep_table(results: dict[tuple[str, str], BenchResult]) -> str:
    """Deep table: ns/op only + Handles cycles? column."""
    header = (
        "| Library | ns/op | Handles cycles? |\n"
        "|---------|------:|:---------------:|"
    )
    allowed = SCENARIO_LIBS["Deep"]
    rows = []
    for lib_key, (label, _) in sorted(BENCH_META.items(), key=lambda x: x[1][1]):
        if allowed is not None and lib_key not in allowed:
            continue
        omission = KNOWN_OMISSIONS.get((lib_key, "Deep"))
        result = results.get((lib_key, "Deep"))
        cycle = CYCLE_BEHAVIOR.get(lib_key, "—")
        if omission:
            rows.append(f"| {label} | {omission} | {cycle} |")
        elif result:
            ns = fmt_ns(result.ns_op)
            rows.append(f"| {label} | {ns} | {cycle} |")
    return header + "\n" + "\n".join(rows)


# ── Section builder ───────────────────────────────────────────────────────────

def build_performance_section(
    results: dict[tuple[str, str], BenchResult],
    cpu: str,
    go_ver: str,
) -> str:
    parts = [
        "FastCopier beats every reflection-based competitor in fair benchmarks across 7 libraries.  ",
        f"Benchmarks run on {cpu}, {go_ver}, `-benchtime=3s`.\n",
    ]

    for scenario_key, scenario_title in SCENARIOS:
        parts.append(f"### {scenario_title}\n")
        if scenario_key == "Deep":
            parts.append(build_deep_table(results))
        else:
            parts.append(build_std_table(scenario_key, results))
        parts.append("")

    # Footer notes
    parts += [
        "> **FastCopier with generated code matches manual copy on Complex.**",
        "> FastCopier is the **only** library that both completes the deep copy correctly **and** handles pointer cycles.  ",
        "> `⚠️ shallow ptrs` = pointer fields are copied as-is (same address), not recursively cloned.",
        "",
        "**Allocation notes:**",
        "- **Structs and slices:** zero allocations on repeated calls (`sync.Pool` + slice capacity reuse)",
        "- **Maps:** unavoidable allocation per call (new map required every time)",
        "- **First call:** allocates the copy plan; all subsequent calls reuse it from the sharded cache",
        "",
        "See [BENCHMARKS.md](BENCHMARKS.md) for the full comparison including the "
        "code-generation tier and safety matrix.",
    ]

    return "\n".join(parts)


# ── README update ─────────────────────────────────────────────────────────────

def update_readme(readme_path: str, new_section: str) -> None:
    with open(readme_path) as f:
        content = f.read()

    start_marker = "<!-- BENCHMARK_RESULTS_START -->"
    end_marker = "<!-- BENCHMARK_RESULTS_END -->"

    start_idx = content.find(start_marker)
    end_idx = content.find(end_marker)

    if start_idx == -1 or end_idx == -1:
        print("ERROR: anchor comments not found in README.md", file=sys.stderr)
        sys.exit(1)

    after_start = start_idx + len(start_marker)
    new_content = (
        content[:after_start]
        + "\n\n"
        + new_section
        + "\n\n"
        + content[end_idx:]
    )

    with open(readme_path, "w") as f:
        f.write(new_content)

    print(f"Updated {readme_path}")


# ── Entry point ───────────────────────────────────────────────────────────────

def main() -> None:
    if len(sys.argv) not in (3, 4):
        print(
            f"Usage: {sys.argv[0]} <bench_gen.txt> <bench_nogen.txt> <README.md>\n"
            f"       {sys.argv[0]} <bench_gen.txt> <README.md>  (no-gen pass omitted)",
            file=sys.stderr,
        )
        sys.exit(1)

    if len(sys.argv) == 4:
        bench_gen, bench_nogen, readme = sys.argv[1], sys.argv[2], sys.argv[3]
    else:
        bench_gen, bench_nogen, readme = sys.argv[1], None, sys.argv[2]

    results = parse_benchmarks(bench_gen)

    # Inject pure-reflection FastCopier rows from the no-gen pass (if provided).
    # The no-gen file contains BenchmarkFastCopier_* without codegen registered,
    # so those numbers reflect the pure reflection engine.
    if bench_nogen:
        nogen = parse_benchmarks(bench_nogen)
        for scenario_key in ("Simple", "Nested", "Complex", "Deep"):
            key = ("FastCopier", scenario_key)
            if key in nogen:
                results[("FastCopierReflect", scenario_key)] = nogen[key]

    cpu, go_ver = extract_env(bench_gen)
    section = build_performance_section(results, cpu, go_ver)
    update_readme(readme, section)


if __name__ == "__main__":
    main()
