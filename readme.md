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

Commands that act on a document take its kind, then a target: `awf <command> <kind> <target>`. The kind is `workflow` or `task`. The target is `<id>[@<version>]` in the store, or `-f <path>` for a file. An unpinned `<id>` resolves to its latest version.

| Command | Action |
| --- | --- |
| `awf new <kind> <id>[@<version>]` | Create a document, or a new version of one. |
| `awf ls [<kind>]` | List the documents in the store. |
| `awf validate <kind> <target>` | Check a document against the spec. |
| `awf seal <kind> <target>` | Validate a document and write its sha. A sealed document needs a new version to change. |
| `awf bundle workflow <target>` | Inline a workflow's `$ref` tasks into one document. |
| `awf render <kind> <target>` | Open a document as an HTML page in the browser. |
| `awf ui` | Browse the store's workflows in the browser. |
| `awf lsp` | Run the OpenAWF language server over stdio. |

`awf <command> --help` lists every flag.

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