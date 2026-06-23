# Delta Spec: Crypto+Niche+BEAM Audit

## Requirement: Language Audit (Oleada 6)

The audit system MUST support code auditing for Solidity, Erlang, Dart, Julia, and Nim. Execution MUST correctly reflect standard output for valid code, and MUST capture standard error and non-zero exit codes for invalid syntax or compilation errors.

### Scenario: Niche execution
- GIVEN a valid Erlang, Dart, Julia, or Nim snippet
- WHEN the respective sandbox executes it
- THEN it MUST complete successfully
- AND stdout MUST match the expected output

### Scenario: Solidity compilation
- GIVEN a valid Solidity snippet
- WHEN the Solidity sandbox executes it
- THEN it MUST complete successfully without compilation errors
