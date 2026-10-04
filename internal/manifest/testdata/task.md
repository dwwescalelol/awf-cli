---
openawf: 0.1.0
version: 0.1.0
sha: null
summary: Produce a diff for the branch.
input:
  branch: string
output:
  diff: string
model: opus
outcomes:
  - ok
  - fail
uses:
  - fs
  - gh
---

# create-diff

Describe the work this task performs, and what it returns.
