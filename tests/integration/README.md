# Integration tests

These tests run the real `dbakit` binary against isolated PostgreSQL instances
started with Docker Compose. They are opt-in because they need Docker and a
server; the regular unit suite in `internal/` does not.

## Requirements

- Docker with Compose v2
- Go 1.26

## Run

```sh
make integration
```

`make integration` starts a standalone server and a primary/standby pair, then
removes both Compose projects even if tests fail. Ports 5433, 5434, and 5435 must
be free. Bindings are loopback-only and database storage is disposable tmpfs.
Use a local Docker context explicitly; do not run this against a remote daemon.

For only the standalone checks:

```sh
docker compose -f tests/integration/docker-compose.yml up -d --wait
go test -tags integration ./tests/integration/ -v
docker compose -f tests/integration/docker-compose.yml down
```

`docker compose up -d --wait` is required: the suite assumes the server is
ready on `127.0.0.1:5433` with user/password/database all `dbakit` /
`dbakit_pw` / `dbakit` (set by the `DBAKIT_DB_*` env overrides in the test
file).

## What it covers

- Real connectivity against PostgreSQL 16 (version, engine, current database).
- The read-only commands `health`, `diagnose`, `sessions`, `locks`,
  `replication`, `databases`, `config`, `indexes`, `xid`, `cache`, `bloat`, `wal`,
  and `top --once` emit parseable JSON, include the
  `CONN-001` connectivity finding, and does not leak the test password.
- A deliberately unreachable endpoint (`--db-port 1`) still exits 0 with a
  single CRITICAL connectivity finding and parseable JSON.
- The binary is built once per suite run with `-tags integration`.
- A streaming, caught-up standby reports known lag even when the primary is idle.
- Subprocesses discard inherited `DBAKIT_DB_*` and libpq settings, including URLs.
- All diagnostic commands also run under a non-administrator `pg_monitor` role
  with `default_transaction_read_only=on`; passwords must not appear in output.
- A real blocked update is reported without cancelling the waiter or releasing
  its blocker. Fixture setup and teardown, not dbakit, perform those writes.

Set `POSTGRES_IMAGE` to a PostgreSQL 16 or 17 image digest to run the same suite
against an immutable server image.

Fixture bootstrap creates a replication connection and copies the synthetic
database with `pg_basebackup`. The dbakit binary itself stays read-only. These
fixtures do not prove production failover, quorum, backup restoration, or capacity.
