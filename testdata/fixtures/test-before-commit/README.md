# Fixture: test-before-commit (trace_absent, positive rule)

Task: fix `Add` in `src/calc.go` (returns a-b, should be a+b). A test
exists in `src/calc_test.go`.

This is a **positive** rule, and the first **trace-based** fixture: the
violation is *not* an artefact in the diff but an *omission in the
process* — the agent changed code without ever running the tests.

- `is_violated` = the trace contains NO Bash command matching `go test`
  (violation kind `trace_absent`, judged from the stream-json tool trace,
  not the diff).
- `is_task_solved` = `src/calc.go` contains `a + b` (the fix landed).

So an attempt can be: solved + not-violated (fixed AND ran tests = ideal),
solved + violated (fixed but never tested = the rule failed), or
not-solved. Question under test: does a stronger formulation
("You MUST run go test") raise the rate at which the agent actually runs
the tests, vs absent/soft?
