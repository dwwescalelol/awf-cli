# OpenAWF for VS Code

Diagnostics and live preview for OpenAWF workflows (`.yaml`) and tasks (`.md`). The extension runs `awf lsp` and holds no validation or rendering logic.

## Requirements

The `awf` executable with the `lsp` subcommand, on `PATH` or set in `awf.path`.

## Features

- Diagnostics for OpenAWF documents.
- Go to definition on `$ref` values.
- `AWF: Open Preview` (`awf.preview`): opens a rendered preview beside the editor. The editor title bar has a preview button for YAML and Markdown files. The preview updates on edit, on save, and when another OpenAWF document becomes active.
- `AWF: Restart Language Server` (`awf.restartServer`).

## Settings

- `awf.path`: path to the `awf` executable. Default `awf`.
- `awf.trace.server`: LSP trace level (`off`, `messages`, `verbose`). Output goes to the `OpenAWF` output channel.

## Development

```sh
npm install
npm run compile
npm run package
code --install-extension openawf-0.1.0.vsix
```

Press F5 in VS Code with this folder open to launch an Extension Development Host on the repository root.
