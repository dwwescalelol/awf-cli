import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
} from "vscode-languageclient/node";

export const RENDER_METHOD = "awf/render";

export interface RenderParams {
  uri: string;
}

export interface RenderResult {
  html: string;
}

export function awfPath(): string {
  return vscode.workspace.getConfiguration("awf").get<string>("path") || "awf";
}

export function createClient(output: vscode.OutputChannel): LanguageClient {
  const command = awfPath();
  const serverOptions: ServerOptions = {
    run: { command, args: ["lsp"], transport: TransportKind.stdio },
    debug: { command, args: ["lsp"], transport: TransportKind.stdio },
  };
  const clientOptions: LanguageClientOptions = {
    documentSelector: [
      { scheme: "file", language: "yaml" },
      { scheme: "file", language: "markdown" },
    ],
    outputChannel: output,
    synchronize: {
      configurationSection: "awf",
    },
  };
  return new LanguageClient("awf", "OpenAWF Language Server", serverOptions, clientOptions);
}

export async function startClient(client: LanguageClient): Promise<boolean> {
  try {
    await client.start();
    return true;
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    const choice = await vscode.window.showErrorMessage(
      `OpenAWF: could not start "${awfPath()} lsp". Set awf.path to the awf executable. (${message})`,
      "Open Settings",
    );
    if (choice === "Open Settings") {
      await vscode.commands.executeCommand("workbench.action.openSettings", "awf.path");
    }
    return false;
  }
}
