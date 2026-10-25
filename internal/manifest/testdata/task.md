---
openawf: 0.1.0
version: 0.1.0
summary: Produce a diff for the branch.
input:
  branch: string
model: anthropic/claude-opus-5
outcomes:
  ok:
    type: object
  fail:
    type: object
uses:
  - fs
  - gh
system: You write diffs.
sha: null
---

# create-diff

Describe the work this task performs, and what it returns.
