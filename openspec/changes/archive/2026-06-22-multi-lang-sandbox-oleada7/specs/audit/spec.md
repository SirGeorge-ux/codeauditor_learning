# Delta Spec: Legacy+Niche Audit

## Requirement: Language Audit (Oleada 7)

The audit system MUST support code auditing for PowerShell, Objective-C, F#, Cobol, and Racket. Execution MUST correctly reflect standard output for valid code, and MUST capture standard error and non-zero exit codes for invalid syntax or compilation errors.

### Scenario: Legacy execution
- GIVEN a valid PowerShell, Objective-C, F#, Cobol, or Racket snippet
- WHEN the respective sandbox executes it
- THEN it MUST complete successfully
- AND stdout MUST match the expected output
