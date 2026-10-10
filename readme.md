# awf

`awf` is a command-line implementation of the [OpenAWF Specification](https://github.com/dwwescalelol/OpenAWF-Specification). OpenAWF describes an agentic workflow as a single document: a finite-state machine of tasks that runs from a start task to a terminal one.

`awf` validates, seals, bundles and renders OpenAWF workflows and tasks. It supports spec version 0.1.0.

## Install

```sh
go install github.com/dwwescalelol/awf-cli/cmd/awf@latest
```

## Documents

A workflow is a `.yaml` file with a top-level `openawf` key. A task is a `.md` file whose YAML frontmatter has an `openawf` key, with the task's instructions in the Markdown body. A workflow can reference a task with `$ref`, either by path or as `<id>@<version>`.

## Stores

`awf` keeps documents in a store, a `.awf` directory:

```
.awf/
  wf/<id>/<version>.yaml
  task/<id>/<version>.md
```

The project store is the nearest `.awf` directory above the working directory, searched up to the home directory. The global store is `$AWF_HOME`, or `~/.awf` when it is unset. `--global` selects the global store.

## Commands

| Command | Action |
| --- | --- |
| `awf new workflow <id>[@<version>]` | Create a workflow, or a new version of one. `awf new task` creates a task. |
| `awf ls` | List the workflows and tasks in the store. |
| `awf validate <id>[@<version>]` | Check a document against the spec. `-t` selects a task, `-f <path>` a file. |
| `awf seal workflow <id>[@<version>]` | Validate a document and write its sha. A sealed document needs a new version to change. |
| `awf bundle <id>[@<version>]` | Inline a workflow's `$ref` tasks into one document. |
| `awf render <id>[@<version>]` | Open a document as an HTML page in the browser. |
| `awf ui` | Browse the store's workflows in the browser. |
| `awf lsp` | Run the OpenAWF language server over stdio. |

An unpinned `<id>` resolves to its latest version. `awf <command> --help` lists every flag.

## Editors

`awf lsp` gives editors diagnostics, go-to-definition on `$ref`, and a live preview. [editors/](editors/README.md) covers the VS Code extension and Neovim setup.

## Development

```sh
go test ./...
go build ./cmd/awf
```

## License

MIT. See [LICENSE](LICENSE).

---