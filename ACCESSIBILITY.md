Accessibility
Novera Accessibility Commitment
Novera aims to make its software, documentation, command-line interfaces, dashboards, examples, and project communication usable by as many people as reasonably possible.

Novera is infrastructure software. Its primary interfaces are APIs, CLI tools, configuration, logs, metrics, documentation, and Kubernetes/Linux operational workflows rather than a consumer web application. Accessibility therefore includes both traditional digital accessibility and operational accessibility: an engineer should be able to understand, configure, diagnose, and operate Novera without unnecessary sensory, motor, cognitive, or platform-specific barriers.

This document defines the accessibility expectations for the project.
1. Scope
Accessibility applies to:
- CLI output and interactive commands
- documentation
- README files
- configuration examples
- dashboards and web interfaces, if provided
- Kubernetes resources and operational interfaces
- error messages
- logs and diagnostics intended for humans
- diagrams and architecture documentation
- release notes
- project website, if provided
- contributor and maintainer workflows
Accessibility does not mean weakening security, correctness, isolation, or operator safety.
2. Guiding Principles
Novera follows these principles:
Perceivable
Important information must not depend on one sensory channel alone.
Operable
Interfaces should be usable without requiring precise mouse interaction or a specific input device.
Understandable
Commands, errors, configuration, documentation, and operational states should be clear and predictable.
Robust
Interfaces should work with common assistive technologies and standard platform mechanisms where applicable.
Safe
Accessibility improvements must not bypass authentication, authorization, security controls, or operational safeguards.
3. Standards and References
For user-facing web interfaces, use the current applicable WCAG 2.2 guidance from W3C:

https://www.w3.org/TR/WCAG22/

For semantic HTML and accessible web practices:

https://www.w3.org/WAI/standards-guidelines/

Where accessibility requirements conflict with an infrastructure-specific operational need, document the trade-off.
4. Command-Line Accessibility
The CLI is a first-class Novera interface.
4.1 Do not rely on color alone
Never make meaning depend only on color.

Prefer:
STATUS: ERROR
STATUS: OK
Color may supplement the text.
4.2 Support plain output
Commands should provide a non-decorated mode where practical.

Example:
novera status --plain
or:
novera status --output=json
Avoid forcing ANSI color, spinners, animations, or terminal-specific control sequences.
4.3 Machine-readable output
Operational commands should provide stable machine-readable formats where appropriate:
json
yaml
Machine-readable output must not depend on terminal color or visual formatting.
4.4 Screen-reader-friendly terminal output
Use clear labels rather than visual alignment.

Prefer:
Scheduler status: healthy
Leader: node-02
Telemetry status: degraded
Telemetry age: 14 seconds
over icon- or color-only status.
4.5 Interactive prompts
Interactive commands should:
- clearly identify the requested input
- avoid unnecessary time limits
- allow cancellation
- explain invalid input
- avoid requiring rapid interaction
- provide a non-interactive mode where practical
5. Keyboard Accessibility for Web Interfaces
If Novera provides a web UI:
- all functionality should be keyboard usable
- Tab/Shift+Tab navigation must follow a logical order
- Enter/Space must activate appropriate controls
- Escape should close dialogs where appropriate
- focus must remain visible
- keyboard traps are prohibited
6. Focus Visibility
Interactive elements must have an obvious focus indicator.

Do not remove default browser focus styles unless an equally or more visible alternative is provided.
7. Color and Contrast
Do not communicate system state using color alone.

Examples:
Healthy
Degraded
Unknown
Unsafe
should be represented textually and, where useful, with icons or patterns.

For web interfaces, validate contrast against the applicable WCAG 2.2 requirements.
8. Motion and Animation
Avoid unnecessary animation.

Where animation exists:
- do not use rapidly flashing content
- do not require animation to understand system state
- respect reduced-motion preferences where applicable
9. Typography and Layout
Documentation and web interfaces should use:
- readable text
- sufficient spacing
- clear headings
- short sections
- consistent terminology
- responsive layouts
- reflow on smaller screens
Do not encode important information only in tiny text, dense screenshots, or visual-only diagrams.
10. Screen-Reader Semantics
For web interfaces use semantic HTML whenever possible:
<header>
<nav>
<main>
<section>
<article>
<footer>
Use logical heading hierarchy.

Controls must have accessible names.

Informational images must have useful alternative text.

Decorative images should not create unnecessary screen-reader noise.
11. Forms and Configuration
Configuration interfaces should:
- label every input
- identify required fields
- explain valid values
- provide useful error messages
- preserve valid input when validation fails
- associate errors with the relevant field
- avoid relying only on placeholder text
For infrastructure configuration, errors should identify:
field
problem
expected value
example
Example:
Error: telemetry.interval must be >= 100ms.

Received: 10ms
Example: telemetry.interval: 500ms
12. Error Message Accessibility
Errors must be understandable without visual context.

Prefer:
ERROR: Kubernetes API authentication failed.

Action:
Verify the configured service account credentials.

Reference:
docs/troubleshooting/kubernetes-authentication.md
Never expose credentials, tokens, private keys, or sensitive information in errors.
13. Logs
Human-readable logs should not require color.

Prefer structured fields:
level=ERROR
component=scheduler
reason=STALE_TELEMETRY
workload=payment-api
telemetry_age=17s
Machine-readable logs should remain parseable without terminal formatting.
14. Metrics and Dashboards
Metrics must have:
- meaningful names
- documented units
- descriptions
- clear labels
- predictable semantics
Dashboards must not rely solely on color.

Prefer states such as:
Healthy — 0 SLO violations
Degraded — telemetry stale
Unknown — telemetry unavailable
Unsafe — scheduling blocked
15. Architecture Diagrams
Architecture diagrams are supplementary documentation, not the only source of information.

Every important diagram should have a textual explanation covering:
- components
- data flow
- trust boundaries
- important dependencies
16. Images and Screenshots
Images that communicate information should include useful alternative text.

Avoid screenshots containing essential information that is not also reproduced in text.

For screenshots, document:
- what is being demonstrated
- relevant command/output
- expected result
- important configuration
17. Documentation Structure
Documentation should use:
- descriptive headings
- consistent terminology
- short sections
- explicit prerequisites
- numbered procedures where order matters
- text alternatives to diagrams
- descriptive link text
Avoid link text such as:
click here
read this
more
Prefer descriptive link text.
18. Cognitive Accessibility
Operational documentation should minimize unnecessary cognitive load.

Prefer:
Problem
Cause
Evidence
Action
Verification
Rollback
For complex procedures provide:
1. prerequisites
2. command
3. expected output
4. failure cases
5. rollback
19. Terminology
Use consistent names for system states.

Recommended Novera state vocabulary:
HEALTHY
DEGRADED
STALE
UNKNOWN
UNAVAILABLE
UNSAFE
Do not use multiple terms for the same state without documenting the distinction.

Avoid unexplained abbreviations on first use.

Example:
SLO (Service Level Objective)
20. Internationalization
Where user-facing text exists, design so localization can be added later.

Avoid:
- hard-coded text embedded in layout assumptions
- sentence concatenation that assumes English word order
- assumptions about text length
- locale-insensitive dates/times
Infrastructure identifiers and configuration keys should remain language-neutral.
21. Time and Data Presentation
Where timestamps are shown:
- use an unambiguous format
- identify timezone where relevant
- do not rely only on relative time
Prefer:
2026-11-03T22:30:15Z
and, where useful:
Last telemetry received: 14 seconds ago
22. Accessibility and Security
Accessibility must not weaken security.

Do not:
- disable authentication to simplify access
- expose hidden privileged controls
- expose secrets by default
- bypass confirmation for destructive operations
- remove authorization boundaries
- expose sensitive audit data unnecessarily
Provide accessible ways to perform the same authorized operation securely.
23. Accessibility and Automation
Machine-readable interfaces are also useful for:
- screen-reader-friendly workflows
- scripted operations
- CI/CD
- remote administration
- users who cannot interact with dynamic terminal interfaces
Provide stable JSON/YAML/plain-text outputs where appropriate.

Avoid requiring automation to parse decorative human-formatted output.
24. Accessibility Testing
CLI
Test:
- color-disabled operation
- plain output
- narrow terminal widths
- machine-readable output
- screen-reader-compatible terminal environments where available
Web UI
Where present, test:
- keyboard-only operation
- focus order
- screen readers
- zoom/reflow
- contrast
- reduced motion
- accessibility tree
- automated accessibility checks
Automated testing supplements but does not replace manual accessibility testing.
25. CI Accessibility Checks
Where a web UI exists, add suitable automated checks for:
- missing labels
- missing accessible names
- focus issues
- contrast issues
- heading structure
- common WCAG violations
Do not treat an automated scanner as the sole accessibility assessment.
26. Accessibility Issue Reporting
Accessibility issues should use the normal project issue process unless they are security-sensitive.

Reports should contain:
environment
interface
steps to reproduce
expected behavior
actual behavior
assistive technology if relevant
Do not include production secrets or sensitive information.
27. Maintainer Review Checklist
For user-facing changes, ask:
- Can this be used without a mouse?
- Does it work without color?
- Does it work without animation?
- Is focus order logical?
- Are errors understandable?
- Is important information available as text?
- Does the CLI have plain/machine-readable output?
- Are screen readers given meaningful labels where relevant?
- Are diagrams accompanied by text?
- Have security controls been preserved?
- Has accessibility behavior been tested?
28. Definition of Done
A user-facing change is accessibility-complete when:
- semantics are correct
- keyboard operation works where applicable
- color is not the sole information channel
- errors are understandable
- important information has a textual representation
- CLI output works without color
- machine-readable output exists where operationally appropriate
- documentation is understandable
- relevant automated checks pass
- appropriate manual checks are complete
- security boundaries remain intact
29. Current Novera Scope
Initial interfaces are expected to be:
CLI
APIs
Kubernetes resources
configuration
logs
metrics
documentation
A web UI is optional.

If a web UI is introduced, the web-specific requirements in this document become mandatory.
30. Accessibility as Systems Quality
Novera considers accessibility part of the same engineering discipline as:
security
reliability
performance
observability
operability
maintainability
An infrastructure tool is not fully usable if an operator cannot understand its state, diagnose errors, execute commands, or access important information through their available interface.
Make the safe, correct operation of Novera understandable and usable without requiring a specific sensory, motor, or interaction method.

References
- W3C Web Content Accessibility Guidelines (WCAG) 2.2: https://www.w3.org/TR/WCAG22/
- W3C Accessibility Standards Overview: https://www.w3.org/WAI/standards-guidelines/
