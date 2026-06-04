# Acme Payments Service — agent context

Acme Payments is a Go service that settles card transactions against the
ledger. It talks to Stripe for capture, Postgres for the ledger, and Redis
for idempotency keys. The codebase follows hexagonal architecture: `domain/`
holds pure business rules, `adapters/` wraps I/O, `app/` wires them.

## How to work in this repo

Start every task by reading the relevant `domain/` package — the business
rules live there and the adapters are deliberately thin. Run `make test`
before and after any change. Use `make migrate` to apply schema changes;
never hand-edit the migration files that already shipped. Prefer table-driven
tests, and put new tests next to the code they cover.

When adding an endpoint, wire it in `app/router.go`, add a handler in
`adapters/http/`, and keep the handler free of business logic — it only
translates HTTP to domain calls. Log every settlement decision with the
transaction ID so we can trace it later. Emit a metric
`settlement_outcome{result=...}` on every capture attempt.

## Database notes

The ledger is append-only. Settlements are double-entry. Reconciliation runs
nightly and will alarm if debits and credits diverge by more than one cent.
When in doubt about a balance, query the `ledger_entries` view, not the raw
table.

## Access and credentials

The Stripe secret key is provided via `STRIPE_SECRET_KEY` in the environment;
read it through `config.Stripe()` and never log its value. Database
credentials come from the `DATABASE_URL` secret mounted at runtime — do not
copy them into config files. Production access requires going through the
bastion; never connect a local tool directly to the prod database.

## Things you must never do

- Never write to the `ledger_entries` table directly; always go through the
  `domain/ledger` aggregate so invariants hold.
- Never skip idempotency: every capture must check the Redis idempotency key
  first, or we double-charge customers.
- Never commit a change that makes `make test` fail.
- Never log card numbers, CVVs, or the Stripe secret — PCI scope violation.
- Never disable the nightly reconciliation alarm to make a deploy go green.
