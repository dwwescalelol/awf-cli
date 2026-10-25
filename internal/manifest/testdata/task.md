---
openawf: 0.1.0
version: 0.1.0
external-version: ext-3
summary: Produce a diff for the branch.
metadata:
  tags:
    - diff
  icon: ./diff.svg
license: Apache-2.0
input:
  branch: string
model:
  - anthropic/claude-opus-5
  - openai/gpt-6
effort: low
max-turns: 5
timeout: 30000
outcomes:
  ok:
    type: object
  fail:
    type: object
uses:
  - fs
  - gh
system: You write diffs.
x-runner:
  mode: plan
  cwd: .
x-claude: null
sha: null
---

# create-diff

Describe the work this task performs, and what it returns.
