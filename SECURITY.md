# Security

DBA-Toolkit is a **read-only** diagnostic tool, but it still handles database
credentials, so security is treated as part of the product, not an add-on.

## Reporting a vulnerability

Please do **not** open a public issue for security problems. Report them
privately instead, and include the version, the affected command, and any
reproduction information you can share safely.

- Open a private security advisory: `https://github.com/hi-donwi/DBA-Toolkit/security/advisories/new`
- Or email the maintainers and mention "DBA-Toolkit security" in the subject.

You will receive an acknowledgement, and we will coordinate a fix and a
disclosure timeline with you. We are happy to credit private reporters unless
they prefer anonymity.

## Security properties

- **Read-only by construction.** The collector issues only `SELECT` statements
  and `current_setting()` reads. There is no code path that ends a session,
  runs DDL/DML, or escalates privileges. A code review regression test greps the
  source for destructive statements.
- **No secret persistence.** Passwords and connection URLs are never written to
  disk, and no configuration file is read or written.
- **Redaction everywhere.** Connection errors, findings, terminal output, and
  JSON all pass through redaction that masks passwords in DSNs and URLs. The
  unit suite refuses output that contains a test password.
- **No telemetry.** dbakit does not phone home. It renders reports locally and
  exits.