import * as path from "path";
import * as vscode from "vscode";
import { LanguageClient } from "vscode-languageclient/node";

export class Preview implements vscode.Disposable {
  private panel?: vscode.WebviewPanel;
  private uri?: string;
  private timer?: NodeJS.Timeout;
  private seq = 0;
  private readonly subscriptions: vscode.Disposable[];

  constructor(private readonly client: () => LanguageClient | undefined, documents: Set<string>) {
    this.subscriptions = [
      vscode.workspace.onDidChangeTextDocument((e) => {
        if (e.document.uri.toString() !== this.uri) return;
        clearTimeout(this.timer);
        this.timer = setTimeout(() => this.refresh(), 300);
      }),
      vscode.window.onDidChangeActiveTextEditor((editor) => {
        const uri = editor?.document.uri.toString();
        if (uri && uri !== this.uri && documents.has(uri)) this.render(uri);
      }),
    ];
  }

  show(uri: string): void {
    if (this.panel) {
      this.panel.reveal(vscode.ViewColumn.Beside, true);
    } else {
      this.panel = vscode.window.createWebviewPanel("awf.preview", "AWF Preview", { viewColumn: vscode.ViewColumn.Beside, preserveFocus: true }, { enableScripts: true, localResourceRoots: [] });
      this.panel.onDidDispose(() => {
        clearTimeout(this.timer);
        this.panel = this.uri = undefined;
      });
    }
    this.render(uri);
  }

  refresh(): void {
    if (this.uri) this.render(this.uri);
  }

  dispose(): void {
    clearTimeout(this.timer);
    this.panel?.dispose();
    this.subscriptions.forEach((s) => s.dispose());
  }

  private async render(uri: string): Promise<void> {
    const panel = this.panel;
    if (!panel) return;
    this.uri = uri;
    const client = this.client();
    if (!client) return;
    const seq = ++this.seq;
    try {
      const { html } = await client.sendRequest<{ html: string }>("awf/render", { uri });
      if (seq !== this.seq || panel !== this.panel) return;
      panel.title = `Preview ${path.basename(vscode.Uri.parse(uri).fsPath)}`;
      panel.webview.html = html;
    } catch (err) {
      if (seq === this.seq) vscode.window.setStatusBarMessage(`AWF preview: ${err instanceof Error ? err.message : err}`, 5000);
    }
  }
}
