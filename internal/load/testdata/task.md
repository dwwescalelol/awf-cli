---
openawf: 0.1.0
version: 0.1.0
sha: null
summary: Produce a diff for the branch.
input:
  branch: string
model: anthropic/claude-opus-5
outcomes:
  ok:
    type: object
  fail:
    type: object
system: You write diffs.
uses:
  - fs
  - gh
x-runner:
  mode: plan
---

# create-diff

Describe the work this task performs, and what it returns.
