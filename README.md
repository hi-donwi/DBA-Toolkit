# DBA-Toolkit

A small, **read-only** command-line toolkit for PostgreSQL diagnostics, health
checks, and troubleshooting. The binary is `dbakit`.

It collects normalized data with a fixed set of SQL probes, runs a compiled-in
rule catalog, and reports findings either as concise terminal output or as
stable JSON for scripts and monitoring.

> **Safety:** dbakit never modifies the database. It issues only `SELECT`
> statements and `current_setting()` reads. It does not terminate or cancel
> sessions, and it never writes, migrates, or restarts anything. When a
> suspicious condition is found it only recommends an action — it never takes
> it.
>
> **Secrets:** passwords and connection URLs are redacted everywhere, including
> failed-connection error messages and every report format.

## Requirements

- Go 1.26 (build only; dbakit is a single static binary)
- A PostgreSQL 13+ server to inspect (the binary itself has no dependencies)

## Install

```sh
go install github.com/hi-donwi/DBA-Toolkit/cmd/dbakit@latest
# or, from the checkout:
make build          # writes ./dbakit
```

## Quick start

```sh
export DBAKIT_DB_HOST=db1.internal DBAKIT_DB_USER=dba DBAKIT_DB_PASSWORD=...
dbakit health                 # connectivity, latency, uptime, connection usage
dbakit diagnose               # health + long queries + locks + replication
dbakit sessions               # active sessions, longest-running first
dbakit locks                  # blocked sessions and who is blocking them
dbakit replication            # role and replica replay lag
dbakit databases              # size, owner, and connection count per database
dbakit indexes                # user indexes with size, scans, and validity
dbakit xid                    # transaction ID age, autovacuum freeze horizon & wraparound
dbakit config                 # a small set of configuration settings
dbakit rules                  # the compiled-in rule catalog
```

Connection details resolve in this order: **flag → environment variable →
default**. Every setting has both a `--db-*` flag and a `DBAKIT_DB_*`
environment variable; `--db-url` (or `DBAKIT_DB_URL`) takes precedence over the
individual settings.

| Setting | Flag | Environment | Default |
|---|---|---|---|
| Host | `--db-host` | `DBAKIT_DB_HOST` | `localhost` |
| Port | `--db-port` | `DBAKIT_DB_PORT` | `5432` |
| Database | `--db-name` | `DBAKIT_DB_NAME` | `postgres` |
| User | `--db-user` | `DBAKIT_DB_USER` | `postgres` |
| Password | `--db-password` | `DBAKIT_DB_PASSWORD` | *(none)* |
| SSL mode | `--db-sslmode` | `DBAKIT_DB_SSLMODE` | `prefer` |
| Connection URL | `--db-url` | `DBAKIT_DB_URL` | *(none)* |

## Commands

| Command | Aliases | What it reports |
|---|---|---|
| `health` | | Connectivity, version, latency, uptime, connection usage, current database size |
| `diagnose` | | `health` plus long queries, locks, and replication |
| `sessions` | `activity` | Active sessions with query text, state, and wait events |
| `locks` | | Blocked sessions and their blocking sessions |
| `replication` | `repl` | Role (primary/standby) and replay lag per connected replica |
| `databases` | `dbs` | Per-database size, owner, and connection count |
| `indexes` | `idx`, `index` | User indexes with size, scan count, and validity |
| `xid` | `wraparound`, `freeze`, `vacuum` | Transaction ID (XID) age, wraparound headroom, and oldest tables |
| `config` | | `max_connections`, buffers, WAL level, slow-statement logging, and more |
| `rules` | | The compiled-in rule catalog (ID, group, title) |
| `version` | | Version, commit, and build time |

## Required privileges

dbakit only ever issues `SELECT` statements and `current_setting()` reads, so a
login role with `CONNECT` on the database is enough to run every command.
Two grants make the output useful for a DBA account:

```sql
GRANT pg_read_all_stats TO "monitoring_role";
GRANT CONNECT ON DATABASE app TO "monitoring_role";
```

Without `pg_read_all_stats`, PostgreSQL masks query text for sessions owned by
other roles, and pg_stat_activity reports it as `<insufficient privilege>` —
dbakit surfaces that verbatim rather than guessing. Anything that requires
reading table data (which dbakit does not) is a separate, stricter grant.

## Thresholds

Long-query, lock, replication, index, and XID rules take thresholds; health uses connection
usage percentages.

```sh
dbakit diagnose \
  --conn-usage-warn 75 --conn-usage-critical 90 \
  --long-query-threshold 120s \
  --lock-wait-threshold 30s \
  --lag-threshold 60s --lag-critical 180s
dbakit indexes --unused-min-size 10485760
dbakit xid --warn-age 200000000 --crit-age 1500000000 --top-tables 10
```

Defaults: connection usage warn 80% / critical 95%, long queries 60s, lock
wait 5s, replay lag warn 30s / critical 120s, unused index min size 10MB,
XID age warn 200,000,000 (200M, matches default `autovacuum_freeze_max_age`) / critical 1,500,000,000 (1.5B).

## Output

Configurable worldwide flags:

- `--json` — stable machine-readable report (JSON also includes *PASS* results,
  so consumers decide what to surface).
- `--hide-pass` — in terminal output, suppress *PASS*-severity findings.
- `--no-color` — plain text, no ANSI escapes.
- `--max-query-length` — truncate query text in findings (default 200 runes).
- `--timeout` — per-command connection/session timeout (default 15s).

Findings carry a severity (`CRITICAL`, `WARNING`, `INFO`, `PASS`), a stable
rule ID (e.g. `CONN-001`, `LONGQ-001`, `LOCK-001`, `REPL-002`, `IDX-001`, `IDX-002`, `XID-001`, `XID-002`, `XID-003`), a summary,
evidence, and a recommendation.

### Exit codes

- `0` — the report was produced. A failed connection is a **CRITICAL finding**,
  not a crash, so the JSON stays parseable for monitoring.
- `1` — operational failure (unknown command, invalid argument, timeout while
  running a probe).

## Development

```sh
make fmt      # gofmt
make vet      # go vet
make test     # unit tests (mock-based; no server required)
make cover    # coverage summary
make build    # ./dbakit
```

Unit tests in `internal/` run without a server: collectors parse rows from
fixtures, evaluators and report rendering are pure functions.

Integration tests (build tag `integration`) run the real binary against a
PostgreSQL in Docker Compose:

```sh
make integration
# or manually:
docker compose -f tests/integration/docker-compose.yml up -d --wait
go test -tags integration ./tests/integration/ -v
docker compose -f tests/integration/docker-compose.yml down
```

See `tests/integration/README.md`.

## License

[Apache-2.0](LICENSE).