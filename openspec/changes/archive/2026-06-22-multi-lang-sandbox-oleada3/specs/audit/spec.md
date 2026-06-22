# Delta for Audit

## ADDED Requirements

### Requirement: Systems Language Audit

The system MUST support audit execution for Rust, C, C++, and Zig languages through the sandbox executor.

#### Scenario: Rust audit compilation and execution

- GIVEN valid Rust code and `rustc` is available
- WHEN `Execute(ctx, "rust", code, 30)` is called
- THEN the sandbox MUST run `rustc -o /tmp/out /tmp/code.rs && /tmp/out`
- AND stream stdout and stderr separately

#### Scenario: C audit compilation and execution

- GIVEN valid C code and `gcc` is available
- WHEN `Execute(ctx, "c", code, 30)` is called
- THEN the sandbox MUST run `gcc -o /tmp/out /tmp/code.c && /tmp/out`
- AND stream stdout and stderr separately

#### Scenario: C++ audit compilation and execution

- GIVEN valid C++ code and `g++` is available
- WHEN `Execute(ctx, "cpp", code, 30)` is called
- THEN the sandbox MUST run `g++ -o /tmp/out /tmp/code.cpp && /tmp/out`
- AND stream stdout and stderr separately

#### Scenario: Zig audit compilation and execution

- GIVEN valid Zig code and `zig` is available
- WHEN `Execute(ctx, "zig", code, 30)` is called
- THEN the sandbox MUST run the `sh -c` wrapper that copies source to `/tmp`, compiles with `zig build-exe`, and executes

#### Scenario: Rust compilation error

- GIVEN invalid Rust code with syntax errors
- WHEN `Execute(ctx, "rust", code, 30)` is called
- THEN the sandbox MUST return a non-zero exit code
- AND stderr MUST contain the compiler error message

#### Scenario: C compilation timeout

- GIVEN C code that enters an infinite loop during compilation
- WHEN `Execute(ctx, "c", code, 5)` is called with a 5-second timeout
- THEN the sandbox MUST kill the process
- AND return a timeout error event

#### Scenario: Zig compilation error

- GIVEN invalid Zig code with type errors
- WHEN `Execute(ctx, "zig", code, 30)` is called
- THEN the sandbox MUST return a non-zero exit code
- AND stderr MUST contain the Zig compiler error message

#### Scenario: All 17 language keys valid

- GIVEN valid `AuditRequest` instances for each of the 17 supported languages
- WHEN each is processed
- THEN none MUST be rejected as unsupported
