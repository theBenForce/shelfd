# MADR Architecture Decision Record Manual

This technical manual instructs the AI on proposing, documenting, and updating Architecture Decision Records (ADRs) for Shelfd.

## Invariants
1. Each decision must be recorded as an individual file in `docs/decisions/`.
2. File naming convention: `docs/decisions/XXXX-<kebab-case-title>.md` (e.g. `0001-core-language-and-runtime-go.md`).
3. The master index in `docs/decisions/index.md` must be updated with every addition or status change.
4. Never edit past accepted decisions directly unless explicitly marking them superseded by a new ADR.

## MADR Template Structure
When creating a new decision record, populate the following template:

```markdown
# [Title of Solved Problem and Decided Option]

* Status: [proposed | accepted | rejected | deprecated | superseded by [ADR-000X](000X-title.md)]
* Deciders: [list deciders]
* Date: [YYYY-MM-DD]

## Context and Problem Statement
[Describe the problem context and requirements]

## Decision Drivers
* [Driver 1]
* [Driver 2]

## Considered Options
* [Option 1]
* [Option 2]

## Decision Outcome
Chosen option: "[Option]", because [justification].

### Positive Consequences
* [Advantage 1]

### Negative Consequences
* [Trade-off 1]

## Pros and Cons of the Options
### [Option 1]
* Good, because [...]
* Bad, because [...]
```
