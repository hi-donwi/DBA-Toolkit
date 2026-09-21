# Contributing

Thanks for considering a contribution to DBA-Toolkit.

## Ground rules

- **Stay read-only.** This project's reason to exist is a tool that never
  modifies the database. Proposals that add `pg_terminate_backend`,
  `pg_cancel_backend`, DDL, or DML will be declined.
- **Keep the contract stable.** JSON output shape, rule IDs, and exit-code
  behavior are documented and relied upon by monitoring. Changes to them are
  breaking changes and need a conversation first.
- **Small, atomic commits.** One concern per commit, Conventional Commits
  style (`feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:`).
- **Secrets.** Never include real hostnames, credentials, or client data in
  any issue, commit, or added test fixture. Test passwords are fine, but keep
  them obviously fake.

## Getting started

```sh
git clone git@github.com:hi-donwi/DBA-Toolkit.git
cd DBA-Toolkit
make fmt && make vet && make test
```

All unit tests run without a server. Integration tests are opt-in and need
Docker (`make integration`).

## What to work on

- A failing or missing unit test for existing behavior.
- A new read-only diagnostic rule with an ID, thresholds, tests, and a line in
  `dbakit rules`.
- Documentation fixes that close the gap between README and actual behavior.

## Pull request checklist

- [ ] `make fmt` — no files listed by `gofmt -l`
- [ ] `make vet` — clean
- [ ] `make test` — green
- [ ] New logic has a unit test (fixture-based; no live server)
- [ ] No new destructive SQL, no secrets, no debug leftovers
- [ ] README and `dbakit rules` updated if behavior changed
- [ ] JSON output shape unchanged unless explicitly announced

If a PR touches collector SQL, extend the fixture-backed tests so the query
text stays exercised without a database.