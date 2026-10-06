import * as path from "path";
import * as vscode from "vscode";
import { LanguageClient, State } from "vscode-languageclient/node";
import { RENDER_METHOD, RenderParams, RenderResult } from "./client";

const DEBOUNCE_MS = 300;

export function isCandidate(doc: vscode.TextDocument): boolean {
  return doc.uri.scheme === "file" && (doc.languageId === "yaml" || doc.languageId === "markdown");
}

export class Preview implements vscode.Disposable {
  private panel: vscode.WebviewPanel | undefined;
  private uri: vscode.Uri | undefined;
  private timer: NodeJS.Timeout | undefined;
  private seq = 0;
  private readonly disposables: vscode.Disposable[] = [];

  constructor(private readonly getClient: () => LanguageClient | undefined) {
    this.disposables.push(
      vscode.workspace.onDidChangeTextDocument((e) => {
        if (this.isCurrent(e.document)) {
          this.schedule(e.document);
        }
      }),
      vscode.workspace.onDidSaveTextDocument((doc) => {
        if (this.isCurrent(doc)) {
          this.render(doc, false);
        }
      }),
      vscode.window.onDidChangeActiveTextEditor((editor) => {
        if (this.panel && editor && isCandidate(editor.document) && !this.isCurrent(editor.document)) {
          this.render(editor.document, true);
        }
      }),
    );
  }

  show(doc: vscode.TextDocument): void {
    if (!this.panel) {
      this.panel = vscode.window.createWebviewPanel(
        "awf.preview",
        "AWF Preview",
        { viewColumn: vscode.ViewColumn.Beside, preserveFocus: true },
        { enableScripts: true, retainContextWhenHidden: true },
      );
      this.panel.onDidDispose(() => {
        this.panel = undefined;
        this.uri = undefined;
        this.clearTimer();
      }, null, this.disposables);
    } else {
      this.panel.reveal(vscode.ViewColumn.Beside, true);
    }
    this.uri = doc.uri;
    this.render(doc, false);
  }

  refresh(): void {
    if (!this.panel || !this.uri) {
      return;
    }
    const doc = vscode.workspace.textDocuments.find((d) => d.uri.toString() === this.uri!.toString());
    if (doc) {
      this.render(doc, false);
    }
  }

  dispose(): void {
    this.clearTimer();
    this.panel?.dispose();
    vscode.Disposable.from(...this.disposables).dispose();
  }

  private isCurrent(doc: vscode.TextDocument): boolean {
    return !!this.panel && !!this.uri && doc.uri.toString() === this.uri.toString();
  }

  private schedule(doc: vscode.TextDocument): void {
    this.clearTimer();
    this.timer = setTimeout(() => this.render(doc, false), DEBOUNCE_MS);
  }

  private clearTimer(): void {
    if (this.timer) {
      clearTimeout(this.timer);
      this.timer = undefined;
    }
  }

  private async render(doc: vscode.TextDocument, onlyIfOpenAWF: boolean): Promise<void> {
    const panel = this.panel;
    if (!panel) {
      return;
    }
    const seq = ++this.seq;
    const client = this.getClient();
    if (!client || client.state !== State.Running) {
      if (!onlyIfOpenAWF) {
        this.adopt(doc);
        panel.webview.html = message("The awf language server is not running.");
      }
      return;
    }
    try {
      const params: RenderParams = { uri: doc.uri.toString() };
      const result = await client.sendRequest<RenderResult>(RENDER_METHOD, params);
      if (seq !== this.seq || this.panel !== panel) {
        return;
      }
      this.adopt(doc);
      panel.webview.html = result.html;
    } catch (err) {
      if (seq !== this.seq || this.panel !== panel || onlyIfOpenAWF) {
        return;
      }
      this.adopt(doc);
      panel.webview.html = message(`Cannot render ${path.basename(doc.uri.fsPath)}: ${errorText(err)}`);
    }
  }

  private adopt(doc: vscode.TextDocument): void {
    this.uri = doc.uri;
    if (this.panel) {
      this.panel.title = `Preview ${path.basename(doc.uri.fsPath)}`;
    }
  }
}

function errorText(err: unknown): string {
  if (err && typeof err === "object" && "message" in err) {
    return String((err as { message: unknown }).message);
  }
  return String(err);
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function message(text: string): string {
  return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline';">
<style>
body { font-family: var(--vscode-font-family); color: var(--vscode-foreground); background: var(--vscode-editor-background); padding: 16px; }
p { color: var(--vscode-descriptionForeground); }
</style>
</head>
<body><p>${escapeHtml(text)}</p></body>
</html>`;
}
