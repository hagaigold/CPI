# CPI Fetcher

A small, dependency-free Go utility that pulls Israel's Consumer Price Index (CPI) from the Central Bureau of Statistics (CBS) API and writes it to a tab-delimited text file. Built for unattended use — it fails loudly if the CPI base ever changes, so it never silently emits values from a different base.

## What it does

- Fetches the **General CPI** series (`id=120010`) from the CBS API.
- Validates that the returned base is still **Average 2024 = 100**; aborts with a clear alert if it isn't.
- Writes one tab-delimited line per data point: write date, reference month, a fixed label, and the index value.
- Cross-compiles to a standalone Windows `.exe` (and Linux binary) with no runtime dependencies.

## Output format

Tab-separated, one row per index point:

```
10/06/26	15/04/26	RG1	105.1
```

| Field | Example | Meaning |
|-------|---------|---------|
| Write date | `10/06/26` | System date when the row was generated (`dd/mm/yy`) |
| Reference date | `15/04/26` | The month the CPI value refers to (`dd/mm/yy`, day fixed at 15) |
| Label | `RG1` | Fixed identifier for the importing system |
| Value | `105.1` | The index value, in the Average-2024 base |

The two dates are deliberately separate: the write date tells you *when the row was produced*, the reference date tells you *which month the figure describes*. Default output file is `cpi.txt`.

## Requirements

- **To build:** Go 1.22+ — or just use the included GitHub Actions workflow, which needs nothing installed locally.
- **To run:** none. The binary is statically linked.
- **Network:** outbound HTTPS to `api.cbs.gov.il`. The build environment does not need network access; only the machine that *runs* the binary does.

## Building

### Via GitHub Actions (no local Go required)

Push the repository. The workflow at `.github/workflows/build.yml` cross-compiles both targets on every push and on manual trigger, then uploads them as an artifact.

1. Open the **Actions** tab and select the latest run.
2. Download the **cpi-binaries** artifact from the run summary — a zip containing `cpi.exe` and `cpi-linux`.

The workflow can also be started on demand via the **Run workflow** button (`workflow_dispatch`).

### Locally

```bash
# current platform
go build -o cpi .

# explicit Windows build from any OS
GOOS=windows GOARCH=amd64 go build -o cpi.exe .
```

## Configuration

Settings live as constants near the top of `main.go`:

| Constant | Default | Purpose |
|----------|---------|---------|
| `expectedBase` | `average 2024` | The base the guard checks for (matched case-insensitively, English) |
| `label` | `RG1` | Value written in the third column |
| `reportDay` | `15` | Day used in the reference date |
| `outFile` | `cpi.txt` | Output file path |

To pull more than the latest month, raise the `count` argument in the `getCPI(120010, 1, "en")` call in `main()`. Each row keeps its own reference month, so a multi-row export stays unambiguous.

## How it talks to the CBS API

A few non-obvious things this tool handles, learned the hard way:

- **Browser User-Agent is required.** The CBS endpoint sits behind a WAF that rejects non-browser clients — a missing or generic agent gets a partial response or a dropped connection. The tool sends a standard browser UA string.
- **Language is pinned to English.** With `lang=en`, the base label comes back as `Average 2024`, which is what `expectedBase` matches. If you switch to Hebrew, update `expectedBase` to match (`ממוצע 2024`), or the guard will false-alarm.
- **The value field.** The index value lives at `currBase.value`; its base description is at `currBase.baseDesc`. Values are read as raw text to preserve the API's exact formatting (no float rounding artifacts).

### API test

```
https://api.cbs.gov.il/index/data/price?id=120010&format=json&download=false&last=2&lang=en
```

## Publication schedule

CBS publishes price indices on the **15th of each month at 18:30** (Israel time), referring to the previous month. If the 15th falls on a Friday, holiday eve, Saturday, or holiday, the release moves to the Friday / eve at 14:00. If you poll on the 15th before the release time, you'll get the prior month's figure — that's the schedule, not a bug. Schedule a run after 18:30, or add a retry.

## When the base changes

CBS rebases the CPI every few years (the Average-2024 base replaced the prior one in January 2025). When that happens, the entire series is restated in the new base, and this tool's guard will halt with:

```
[ALERT] CPI base changed: expected "average 2024", got "<new base>"; review and re-sync before continuing.
```

That's by design — it stops bad data at the source. To migrate: confirm the new base label, update `expectedBase`, and re-pull the full history so stored values are internally consistent. During a transition, the API exposes the same month in both bases (`currBase` and `prevBase`) so you can bridge values across the boundary.

## Project layout

```
.
├── go.mod
├── main.go
└── .github/
    └── workflows/
        └── build.yml
```

## Data source

[Israel Central Bureau of Statistics — Price Indices API](https://www.cbs.gov.il/en/Pages/Api-interface.aspx). Public data, no API key required.

---
*Built with assistance from Claude (Anthropic).*
