import * as path from "path";
import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
} from "vscode-languageclient/node";

let client: LanguageClient | undefined;

export function activate(context: vscode.ExtensionContext) {
  const config = vscode.workspace.getConfiguration("origami");
  const binaryPath = config.get<string>("lsp.path", "origami");

  const serverOptions: ServerOptions = {
    run: {
      command: binaryPath,
      args: ["lsp"],
      transport: TransportKind.stdio,
    },
    debug: {
      command: binaryPath,
      args: ["lsp", "--verbose"],
      transport: TransportKind.stdio,
    },
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [
      { scheme: "file", language: "origami-circuit" },
      { scheme: "file", language: "yaml", pattern: "**/circuits/**/*.yaml" },
      { scheme: "file", language: "yaml", pattern: "**/circuits/**/*.yml" },
    ],
    synchronize: {
      configurationSection: "origami",
    },
  };

  client = new LanguageClient(
    "origami-lsp",
    "Origami Circuit LSP",
    serverOptions,
    clientOptions
  );

  client.start();
  context.subscriptions.push({
    dispose: () => {
      if (client) {
        client.stop();
      }
    },
  });

  const statusBar = vscode.window.createStatusBarItem(
    vscode.StatusBarAlignment.Right,
    100
  );
  statusBar.text = "$(symbol-misc) Origami LSP";
  statusBar.tooltip = "Origami Circuit Language Server";
  statusBar.show();
  context.subscriptions.push(statusBar);
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }
  return client.stop();
}
