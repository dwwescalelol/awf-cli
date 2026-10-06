import * as vscode from "vscode";
import { LanguageClient } from "vscode-languageclient/node";
import { createClient, startClient } from "./client";
import { isCandidate, Preview } from "./preview";

let client: LanguageClient | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  const output = vscode.window.createOutputChannel("OpenAWF");
  const preview = new Preview(() => client);
  context.subscriptions.push(output, preview);

  const start = async (): Promise<void> => {
    client = createClient(output);
    if (await startClient(client)) {
      preview.refresh();
    }
  };

  const restart = async (): Promise<void> => {
    const old = client;
    client = undefined;
    if (old) {
      await old.dispose().catch(() => undefined);
    }
    await start();
  };

  context.subscriptions.push(
    vscode.commands.registerCommand("awf.preview", async (uri?: vscode.Uri) => {
      const doc = uri instanceof vscode.Uri
        ? await vscode.workspace.openTextDocument(uri)
        : vscode.window.activeTextEditor?.document;
      if (!doc || !isCandidate(doc)) {
        vscode.window.showInformationMessage("AWF: open a workflow (.yaml) or task (.md) file to preview.");
        return;
      }
      preview.show(doc);
    }),
    vscode.commands.registerCommand("awf.restartServer", restart),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration("awf.path")) {
        void restart();
      }
    }),
  );

  await start();
}

export async function deactivate(): Promise<void> {
  if (client) {
    const old = client;
    client = undefined;
    await old.dispose().catch(() => undefined);
  }
}
