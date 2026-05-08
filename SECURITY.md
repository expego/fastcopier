# Security Policy

## Supported Versions

FastCopier is currently in initial development. Security fixes are applied to the latest commit on the `master` branch.

| Version | Supported |
|---------|-----------|
| latest (master) | ✅ |

## Reporting a Vulnerability

If you discover a security issue in FastCopier, **please do not open a public GitHub issue**. Instead, contact the maintainer directly:

- Email: *(add your contact email here)*
- Or use [GitHub private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing/privately-reporting-a-security-vulnerability) if enabled on this repository.

Please include:
- A description of the vulnerability and its potential impact.
- Steps to reproduce or a minimal proof-of-concept.
- The Go version and OS you tested on.

You will receive a response within **7 business days**. If the report is accepted, a fix will be published and a CVE requested where appropriate.

## Scope

FastCopier is a pure Go library with no network access, no file system access, and no external runtime dependencies. Realistic security concerns include:

- **Unsafe pointer operations** — `engine.go` uses `unsafe.Pointer` to extract the data pointer from a `reflect.Type` interface value for hashing. This is a read-only operation and does not expose memory to callers.
- **`register.go` uses `unsafe.Pointer`** — `registeredFuncPlan.Copy` uses `unsafe.Pointer` to convert `reflect.Value.Addr()` to a typed pointer. This is sound when `dst` is addressable (guaranteed by the calling convention in `Copy`) and `src` is addressable (the common case). The rare non-addressable path falls back to a safe `Interface()` call.
- **Reflection panics** — all `reflect` operations are guarded; no known panic paths exist for well-formed inputs.
- **Infinite loops / stack overflows** — circular reference detection is built-in; the `visited` map prevents infinite recursion on pointer cycles.

## Out of Scope

- Issues in the `benchmarks/` sub-module (competitor libraries).
- Issues in the `cmd/fastcopier-gen/` code generator that only affect generated code quality (not security).
