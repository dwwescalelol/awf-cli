# OpenAWF for VS Code

Diagnostics and live preview for OpenAWF workflows (`.yaml`) and tasks (`.md`). The extension runs `awf lsp`. Validation and rendering live in the server.

## Server

Each `.vsix` targets one platform and contains the `awf` binary for it at `bin/awf` (`bin/awf.exe` on Windows). The extension starts that binary. The `awf.path` setting overrides it.

## Features

- Diagnostics for OpenAWF documents.
- Go to definition on `$ref` values.
- An `Open Preview` code lens on each OpenAWF document.
- `AWF: Open Preview` (`awf.preview`): opens a rendered preview beside the editor. The editor title bar shows a preview button on OpenAWF documents. The preview re-renders 300 ms after an edit and follows the active editor to other OpenAWF documents.
- `AWF: Restart Language Server` (`awf.restartServer`).

The server decides which documents are OpenAWF documents: a document is one when the server returns a code lens for it.

## Settings

- `awf.path`: path to an `awf` executable. Empty uses the bundled binary. Changing it restarts the server. VS Code ignores a workspace value in untrusted workspaces.
- `awf.trace.server`: LSP trace level (`off`, `messages`, `verbose`). Output goes to the `OpenAWF` output channel.

## Development

```sh
npm install
npm run typecheck
npm run compile
npm run package
```

`npm run compile` bundles `src/` with esbuild into `out/extension.js`. `npm run bundle [target]` cross-compiles `awf` into `bin/`. `npm run package [target...]` builds one `openawf-<target>-<version>.vsix` per target. Targets: `darwin-arm64`, `darwin-x64`, `linux-arm64`, `linux-x64`, `win32-arm64`, `win32-x64`. The default target is the host.

```sh
npm run package darwin-arm64 linux-x64
code --install-extension openawf-darwin-arm64-0.1.0.vsix
```

F5 in VS Code with this folder open compiles the extension, builds `bin/awf`, and launches an Extension Development Host on the repository root.
