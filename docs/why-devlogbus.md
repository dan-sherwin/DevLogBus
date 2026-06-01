# Why DevLogBus Exists

DevLogBus exists because local debugging can turn into a stupid little scavenger
hunt.

One service is logging to stdout. Another is using structured logs. A CLI command
prints something useful once and vanishes. The browser console has the other half
of the story. Linux has the systemd journal. Somewhere in that mess is the cause
and effect you need, but your eyes are bouncing between five terminals and a
browser as if that is a reasonable way to live.

DevLogBus gives that work one local stream.

## What It Is

DevLogBus is a local-first structured log bus for development and active
troubleshooting. It runs a local daemon, accepts records from tools and
applications, keeps a bounded in-memory replay buffer, and exposes browser and
terminal viewers.

It can collect records from:

- Go services through `slog`
- C, .NET/C#, Rust, Java/Kotlin, Node/TypeScript, and Python SDKs
- direct HTTP API calls
- the `devlogbus` CLI
- Linux `journald` through `devlogbus-journal-bridge`
- Chrome tabs through Browser Tap

## Who It Is For

DevLogBus is for developers who need to see local cause and effect across more
than one process.

It fits:

- workstation debugging
- private dev boxes
- trusted lab networks
- CLI and service integration work
- browser plus backend workflows
- short-lived troubleshooting sessions

## Who It Is Not For

DevLogBus is not trying to be a hosted observability platform.

It does not provide:

- production retention
- alerting
- metrics
- distributed tracing
- multi-user auth
- a hosted cloud backend

Use a real production observability stack when that is the job. DevLogBus is for
the local work before that, where you want signal now and do not want to wire a
whole cathedral just to see why a button click made a worker complain.

## Why It Is Different

DevLogBus is intentionally local, boring, and direct.

- The daemon stores records in memory.
- The default endpoints stay on the local machine.
- Browser capture starts only when you attach Browser Tap.
- Sources can be grouped, hidden, blocked, cleared, or expunged.
- Package installs and SDKs exist so the tool is easy to try without cloning the
  repo.
- Verification keys and checksums are available, but the operator decides how
  much ceremony they want.

That last point is not accidental. DevLogBus gives you the tools and the choice.
If you choose the fast path, you own the tradeoff.

In short, piss on the electric fence if you want. Just don't act surprised when
physics files a bug report on your ass.
