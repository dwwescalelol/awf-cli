# awf-cli plan

Reference CLI for the OpenAWF spec — https://github.com/dwwescalelol/OpenAWF-Specification

## Stack

| concern | choice |
|---|---|
| language | Go |
| commands | cobra |
| TUI | Bubble Tea v2 + Lip Gloss v2 + Bubbles v2 |
| schema | JSON Schema 2020-12 (`santhosh-tekuri/jsonschema/v6`) |
| YAML | comment-preserving round-trip (`goccy/go-yaml` AST, or line-patch `sha:`) |
| schema file | embedded via `go:embed` |

## Representations

- **Workflow** — YAML or JSON. Header, `start`, `orchestration`, `tasks`, `mcp`.
- **Task** — `.md` with YAML frontmatter. All schema fields except `body` in frontmatter; `body` is the markdown.

A task file is a fragment, not a document. The spec's "YAML or JSON" line covers workflows only.

## Scopes

```
./.awf/   local   default, walks up ancestors like .git
~/.awf/   global  fallback, and the target outside any project
```

`--global` forces global. Created lazily on first use, never at install time.

## Commands

| cmd | does |
|---|---|
| `validate` | syntax → graph → sha |
| `seal` | bundle → validate → hash → write. The sealed file is self-contained |
| `bundle` | inline external `$ref`s into `tasks` |
| `extract` | split `tasks` to `.md` files, replace with `$ref`s |
| `new` | create a workflow or task in scope, unsealed — a new id, or a new version of one |
| `install` | fetch a document from a git remote by `<id>@<version>` |
| `publish` | commit, tag and push a sealed document to a git remote |
| `propose` | push a change to a review branch and open it for review |
| `remote` | `init` a remote's layout; `ls`, `render`, `diff` its contents and proposals |
| `ls` | installed workflows, or one workflow's tasks + versions |
| `render` | swagger-style HTML, serves on localhost (`--out` for a file) |
| `run` | execute the FSM, live TUI view |
| `-v` | CLI version |

## validate

Three passes, in order.

**1. Structural** — JSON Schema 2020-12.

**2. Graph**

| check | rule |
|---|---|
| start resolves | `start` names a key in `tasks` |
| edges resolve | every edge target is a defined task |
| orchestration ↔ tasks | keys match |
| branch coverage | branch keys ⊆ the task's `outcomes` |
| branch required | task with `outcomes` needs a branch, not a bare edge |
| terminal exists | ≥1 node with `null` |
| reachability | every task reachable from `start` |
| liveness | a terminal is reachable from every task |
| mcp refs | `uses` names a server in `mcp` |
| `$ref`s | resolve, and the target validates |

Liveness catches retry cycles with no exit — nothing else does.

Severity line: **can't proceed = error** (edge to an undefined task), **won't be used = warning** (unreachable task). `--strict` promotes warnings.

**3. Hash** — only when `sha` is non-null. Recompute over canonical content, compare.

## Hashing

Canonical form: **bundle → YAML to JSON → JCS (RFC 8785) → sha256**.

Bundling first is what makes it work. Hashing the document as written would hash a `$ref` string rather than the task content, so `bundle` and `extract` would change the hash of a semantically identical workflow and break `seal`/`verify` across the pair. Bundling first makes them hash-neutral.

Excluded from the hash: `sha` itself, `x-meta` (declared ignored by execution), and the version. The version is a mutable label until its tag is pushed, so including it would change the sha of unchanged content. Two versions with identical content therefore share a sha, and sha to `id@version` is one-to-many.

Tasks hash the same way — frontmatter + `body` as one canonical JSON object.

No separate `lint`. Advisory checks are warnings on `validate`, promoted by `--strict`. Model them as a ruleset with severities (Spectral's design), not hardcoded.

## bundle / extract

`tasks:` is the definition store — the equivalent of OpenAPI `components/schemas`.

- **bundle** — pull external `$ref` content into `tasks`. Orchestration edges stay bare names, unchanged.
- **extract** — the inverse.

Not dereference: nothing is inlined at the use site.

`$ref` is external only. Tasks are name-keyed, so an internal `#/tasks/x` would be a second syntax for `x`.

## new

```
awf new workflow feat-dev          # id is free    → 0.1.0, blank skeleton
awf new workflow feat-dev 0.4.0    # id exists     → copy 0.3.0, sha nulled
awf new task create-diff
```

One verb. Whether the id already exists decides which case it is, so the user never picks. No `add` — everywhere else `add` means putting an existing thing into a set (`cargo add`, `helm repo add`), never creating one.

Noun subcommand, not a flag. `install` detects type from content; `new` has no content yet, so the kind must be stated. Defaults to `--local`. Output passes `validate` immediately.

| state | editable |
|---|---|
| `sha: null` | yes — a draft |
| sealed | no — `seal` refuses to overwrite, `--force` overrides |

Editing a sealed document is a validate **error**, not a warning: the sha cannot match.

`seal` is a precondition of `publish`, not a replacement for it. `publish` commits, tags and pushes; see `remotes`.

## Layout

Name is id. Versions coexist, so a workflow can pin an exact task version.

```
.awf/
  wf/<id>/<version>.yaml
  task/<id>/<version>.md
```

`<id>/` groups versions together for `ls`. No leaf directory per version — a workflow is a single file. Content is immutable once written; nothing is overwritten in place.

ASSUMPTION: extracted tasks land in the shared `task/` tree rather than beside their workflow, so two workflows using the same task version share one copy.

Namespacing (`author@wf-name`) deferred. Worth reserving `@` in ids now so it stays available.

## install

```
awf install github.com/zaz/flows feat-dev@0.3.0
awf install github.com/zaz/flows tasks/create-diff@0.1.0
```

Git plumbing only, no HTTPS raw fetch, no host API, no registry. Resolution is one `git ls-remote` globbing `refs/tags/awf/*/<id>/v<version>`, then a fetch of that commit and a `cat-file` of the blob. Identical on GitHub, GitLab, Bitbucket and a bare repo on a fileshare.

Zero matches is not found. One match recovers the docType from the tag. Two or more is ambiguous and the user must qualify with `<docType>/<id>@<version>`.

Workflow or task is detected from content, not a flag: a workflow has `openawf` and `orchestration` at the top level; a task is frontmatter + body with no `openawf`. Extension is a hint only. Neither shape is a validate error. Both land in scope; a task installed alone is referencable by `$ref` from any local workflow.

On landing: fetch → validate → write to scope. No lockfile. The version is in the coordinate and the `sha` is in the document.

Private repos use the user's existing git credentials.

## Versioning

Exact pins only. No semver ranges, no solver — tasks are leaves, so there is no diamond problem.

No lockfile. The point of versioning is provenance, and the sha is its key, not the version: a version is a mutable label until its tag exists, so a document can be run locally and renumbered before publish. `run` stamps the sha of the workflow and of every task that ran into the run record, carrying versions alongside them. A document with `sha: null` is unsealed and has no provenance.

## TUI

The TUI belongs on `run`, not `render`. `render`'s HTML already covers static document browsing; a TUI copy of it renders the same data twice. The TUI earns its place watching an FSM execute live — current task, outcome, retries, cost.

## Spec dependency

`versions/0.1.0.md` and the generated `schema.json` do not exist yet. Four things need normative prose before another implementation can interop:

1. the hashing rule above
2. `$ref` resolution semantics
3. what a conformant validator must reject vs warn
4. edge and branch rules

Write each decision into `versions/0.1.0.md` as the CLI makes it, rather than up front.
