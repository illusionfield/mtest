# mtest - Modern Meteor TinyTest Orchestrator

![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Rod](https://img.shields.io/badge/Rod-Automation-5333ED?style=for-the-badge)
![License](https://img.shields.io/badge/license-MIT-2F855A?style=for-the-badge)

> A polished, security-first reimagination of [zodern/mtest](https://github.com/zodern/mtest), created to eliminate the maintenance burden and dependency vulnerabilities that have accumulated around the original Node.js CLI.

---

## Contents

- [Why a Reimagining?](#why-a-reimagining)
- [Feature Highlights](#feature-highlights)
- [Quick Start](#quick-start)
- [CLI Reference](#cli-reference)
- [Workflow Deep Dive](#workflow-deep-dive)
- [Parity with the Original Project](#parity-with-the-original-project)
- [Migration Tips](#migration-tips)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [License](#license)

---

## Why a Reimagining?

> [!IMPORTANT]
> The primary motivation behind this Go rewrite is to remedy the vulnerable, end-of-life dependencies that power the legacy `@zodern/mtest` npm package. By leaning on modern Go tooling and actively maintained libraries, the attack surface shrinks while day-to-day DX improves.

- **Security posture**: No more transitive npm chain; Go modules (Rod, Logrus, pflag) are actively patched.
- **Portability**: Ships as a single static binary, making CI/CD distribution trivial.
- **Stability**: System signal handling, process groups, and deterministic logging are first-class citizens.

---

## Feature Highlights

| Area | What’s New | Why It Matters |
| ---- | ---------- | -------------- |
| CLI ergonomics | `pflag`-powered parsing, verbose levels (`-vvv`) | Familiar UX with precise logging control |
| Browser automation | Rod + Chromium headless launch | Mirrors Puppeteer behaviour without npm baggage |
| Output fidelity | Streams Meteor + browser console verbatim, skips sentinel lines | All the insights, none of the noise |
| Graceful shutdown | Cross-platform process group teardown | Prevents orphaned Meteor or Chrome processes |
| Observability | Structured, timestamped logrus output | Parse-friendly logs for CI pipelines |

---

## Quick Start

```bash
# 1. Build the binary
go build

# 2. Run tests for a package
./mtest --package <package name>
```

Need a quick smoke test? A tiny Meteor package lives at `test/dummy`. Point the CLI at it to validate your setup:

```bash
./mtest --package ./test/dummy --once
```

Common options:

- `--once` – exit after the first run (perfect for CI)
- `--release <version>` – pin a specific Meteor release
- `--test-app-path <path>` – point at a custom test harness
- `-v`, `-vv`, `-vvv` – raise verbosity up to debug

---

## CLI Reference

| Flag | Alias | Type | Description |
| ---- | ----- | ---- | ----------- |
| `--package` | | string | **Required.** Meteor package name to execute under `meteor test-packages`. |
| `--release` | | string | Target Meteor release (for reproducible environments). |
| `--test-app-path` | | string | Relative/absolute path to an app folder used as the test harness. |
| `--once` | | bool | Stop after the first test cycle completes. |
| `--inspect` | | bool | Passes `--inspect` to Meteor for live debugging. |
| `--inspect-brk` | | bool | Passes `--inspect-brk` to Meteor (breaks on first line). |
| `--port` | | int | Force a specific port (defaults to an auto-picked free port 10000-11999). |
| `--verbose` | `-v` | count | Increase logging level (repeat flag or use `--verbose=N`). |
| `--version` | `-V` | bool | Print semantic version/commit info and exit. |

---

## Workflow Deep Dive

1. **Port discovery**
   By default `mtest` shuffles ports in the 10000-11999 range and selects a free one, matching the behaviour of the Node CLI’s `get-port` dependency.

2. **Meteor orchestration**
   The tool spawns `meteor test-packages` with your CLI flags, mirroring stdout/stderr exactly, and listens for readiness markers (`10015`, `test-in-console listening`).

3. **Headless verification**
   Once ready, Rod launches Chromium in headless mode with the same sandbox overrides Puppeteer used. Console output (minus the magic sentinel) is streamed straight back to your terminal.

4. **Status polling**
   Every 500 ms the page is evaluated to determine TinyTest completion and failure counts. Results are piped back as exit codes, so CI can fail fast.

5. **Graceful teardown**
   SIGINT/SIGTERM handlers fan out to Meteor and Chromium process groups, protecting local dev cycles and shared CI runners from orphaned processes.

---

## Parity with the Original Project

| Capability | `@zodern/mtest` (Node) | `mtest` (Go) |
| ---------- | --------------------- | ------------ |
| CLI flags | ✔ identical flag set | ✔ |
| Auto port selection | via `get-port` | via native TCP probing |
| Puppeteer driven | ✔ | ➜ Rod-powered |
| Stream test console | ✔ | ✔ (sentinel filtered) |
| Process cleanup | basic `tree-kill` | cross-platform process groups |
| Dependency health | multiple unmaintained npm deps | lean Go module graph |

If you relied on custom Node scripts around the original CLI, drop-in replacement is as simple as swapping the executable call — arguments and behaviour line up 1:1.

---

## Migration Tips

> [!TIP]
> To ensure reproducibility, bake the compiled binary into your project’s toolchain (e.g., commit to an internal bucket or wrap it in a Makefile target).

- Replace `npx mtest` invocations with the compiled Go binary.
- Use the new verbosity levels to mirror any logging you previously captured with `DEBUG=*`.
- Validate that your CI environment still has Chromium/Chrome available; Rod auto-downloads when necessary, but caching the binary speeds things up.

---

## Roadmap

- [ ] JSON output mode for machine parsing
- [ ] Native HTML/JUnit report generation
- [ ] Plug-in hooks before/after Meteor spawn
- [ ] Homebrew/Scoop/Tap packages for one-command installs

Have ideas? Open an issue and join the discussion.

---

## Contributing

1. Fork and clone this repository.
2. `go fmt ./...` and `go test ./...` before proposing changes.
3. Describe behavioural impacts clearly in your PR — backwards compatibility is a priority.

---

## License

MIT © The mtest Contributors. See [`LICENSE`](LICENSE) for details.
