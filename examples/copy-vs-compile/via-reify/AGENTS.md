# AGENTS.md

> Compiled by reify. Edit the Reify source, not this file.

## Payments Service

### Rules
- Never write to the ledger_entries table directly; always go through the domain/ledger aggregate
- Never skip the idempotency check before a capture, or we double-charge customers
- Never commit a change that makes make test fail
- Never log card numbers, CVVs, or the Stripe secret — PCI scope violation
- Never disable the nightly reconciliation alarm to make a deploy go green

### Steps
1. read the relevant domain/ package before changing anything
2. run make test to establish a green baseline
3. wire new endpoints through app/router.go and adapters/http/
4. check the Redis idempotency key before every capture

### Tools
read_file, bash

### Observability

- Trace level: standard
- Metrics: settlement_outcome

### Security

- Filesystem: read-write
- Network: allowlist
- Secrets: STRIPE_SECRET_KEY, DATABASE_URL
