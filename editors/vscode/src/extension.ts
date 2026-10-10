import * as path from "path";
import * as vscode from "vscode";
import { LanguageClient, Middleware } from "vscode-languageclient/node";
import { Preview } from "./preview";

export function activate(context: vscode.ExtensionContext): void {
  const bundled = context.asAbsolutePath(path.join("bin", process.platform === "win32" ? "awf.exe" : "awf"));
  const output = vscode.window.createOutputChannel("OpenAWF");
  const documents = new Set<string>();
  let client: LanguageClient | undefined;
  let pending = Promise.resolve();
  const preview = new Preview(() => client, documents);

  const middleware: Middleware = {
    provideCodeLenses: async (doc, token, next) => {
      const lenses = await next(doc, token);
      const uri = doc.uri.toString();
      const had = documents.has(uri);
      if (lenses?.length) documents.add(uri);
      else documents.delete(uri);
      if (had !== documents.has(uri)) void vscode.commands.executeCommand("setContext", "awf.documents", [...documents]);
      return lenses;
    },
  };

  const restart = () => (pending = pending.then(async () => {
    await client?.dispose().catch(() => undefined);
    const command = vscode.workspace.getConfiguration("awf").get<string>("path") || bundled;
    client = new LanguageClient("awf", "OpenAWF Language Server", { command, args: ["lsp"] }, {
      documentSelector: [{ scheme: "file", language: "yaml" }, { scheme: "file", language: "markdown" }],
      outputChannel: output,
      middleware,
    });
    await client.start().then(() => preview.refresh(), () => undefined);
  }));

  context.subscriptions.push(
    output,
    preview,
    vscode.commands.registerCommand("awf.preview", (arg?: string | vscode.Uri) => {
      const uri = arg?.toString() ?? vscode.window.activeTextEditor?.document.uri.toString();
      if (uri) preview.show(uri);
    }),
    vscode.commands.registerCommand("awf.restartServer", restart),
    vscode.workspace.onDidChangeConfiguration((e) => e.affectsConfiguration("awf.path") && restart()),
    { dispose: () => void pending.then(() => client?.dispose()) },
  );
  void restart();
}
