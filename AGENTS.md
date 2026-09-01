# Repository Engineering Requirements

These instructions apply to every change in this repository. Treat the Go/C/C++
boundary as security-sensitive production code.

## Completion gate

Do not describe a change as complete, safe, or faster unless all applicable
items below have been performed and their results are reported. A tool being
unavailable is not a passing result: record the exact limitation and identify
the CI job or supported platform that must complete the check.

## Benchmark every change

- Before editing runtime code, capture a baseline from the unchanged revision.
- Add or update a focused Go benchmark that exercises the production path being
  changed. Keep benchmark inputs and setup identical between baseline and
  candidate; do not compare different machines, build modes, or workloads.
- Run at least five samples with a meaningful duration, normally:

  ```sh
  go test . -run '^$' -bench '<focused-regexp>' -benchmem -benchtime=2s -count=5
  ```

- Compare distributions with `benchstat` when available. Otherwise report the
  median of all samples and preserve the raw measurements in the task result.
- Report latency, throughput when meaningful, Go allocations, and bytes per
  operation. Remember that `benchmem` does not observe allocations performed by
  C/C++; inspect or measure those separately.
- A performance claim requires repeatable data. Investigate large variance and
  do not select only favorable samples.
- Documentation-only or test-only changes must still state `benchmark: N/A`
  with the reason. Never invent a performance impact for a non-runtime change.

## Required correctness and memory-safety validation

Run the narrow tests while iterating, then run the broadest applicable suite:

```sh
go test . -run '^Test' -count=1
go test -race . -run '^Test' -count=1
GOEXPERIMENT=cgocheck2 go test . -run '^Test' -count=1
go test --tags leakcheck . -run '^Test' -count=1
```

Also run `go test ./...` when the local platform can link every package. Use
sanitizers or Valgrind on a supported platform for changes involving native
allocation, pointer arithmetic, buffers, finalization, or ownership. Do not
claim sanitizer coverage when the tool or target does not support it.

Add regression tests for every corrected failure mode. Exercise success,
empty/nil input, invalid input, error returns, repeated calls, cleanup, and
boundary sizes when they are relevant. A crash, fatal V8 check, unexpected
panic, race report, leak, use-after-free, double-free, or cgo pointer violation
blocks completion.

## Go/C/C++ ownership invariants

For every boundary change, explicitly determine and preserve:

- who allocates, owns, borrows, retains, releases, and may mutate each object;
- whether a pointer may outlive the cgo call;
- whether memory contains Go pointers;
- which context and isolate own each V8 handle;
- behavior after `Release`, `Close`, `Dispose`, termination, and error returns.

C/C++ must not retain a pointer into Go memory after a cgo call. For borrowed Go
buffers copied synchronously by C++, keep the Go object alive through the call
with `runtime.KeepAlive`. If native code must retain data, allocate native-owned
storage and define its matching release path. Pair every native allocation and
persistent V8 handle with exactly one release on success and all error paths.
Never release borrowed memory or expose a pointer to stack storage beyond its
lifetime.

Treat every `MaybeLocal`, nullable pointer, and allocation as fallible. Check it
before dereference. Avoid `ToLocalChecked`, unchecked casts, `strlen` on
untrusted/null pointers, and narrowing length conversions unless the input is
provably valid. Return a Go error or V8 error for recoverable invalid input;
do not introduce a panic. Preserve existing documented panic contracts unless
the user explicitly requests an API change.

## Compatibility and production impact

- Preserve exported Go API behavior and native C ABI unless a breaking change
  is explicitly requested. Prefer additive native entry points plus wrappers
  for existing symbols.
- Confirm that the optimized code is reached by the existing production API;
  benchmark-only helpers do not count as production impact.
- Run `gofmt`, the repository's `go generate` formatting check when C/C++ files
  change, `go vet`, and `git diff --check` before completion.
- In the final report, list commands run, pass/fail status, benchmark baseline
  and candidate data, ownership reasoning, compatibility impact, and any checks
  deferred to CI.
