import * as vscode from 'vscode';
import { spawn, spawnSync } from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';
import * as https from 'https';
import * as crypto from 'crypto';
import * as stream from 'stream';
import { promisify } from 'util';
const pipeline = promisify(stream.pipeline);

interface CliIssue {
  file: string;
  line: number;
  // column removed in CLI output; extension will map issues to full-line ranges
  rule: string;
  severity: string;
  message: string;
}

let diagnosticCollection: vscode.DiagnosticCollection;
let outputChannel: vscode.OutputChannel;
let debounceTimer: NodeJS.Timeout | undefined;
const DEBOUNCE_MS = 500;
// Keep last-run issues for hover provider
const lastIssues: Map<string, CliIssue[]> = new Map();
let extContext: vscode.ExtensionContext | undefined;

export function activate(context: vscode.ExtensionContext) {
  extContext = context;
  diagnosticCollection = vscode.languages.createDiagnosticCollection('cadence-workflow-linter');
  context.subscriptions.push(diagnosticCollection);
  outputChannel = vscode.window.createOutputChannel('Cadence Workflow Linter');
  context.subscriptions.push(outputChannel);
  outputChannel.appendLine('Cadence Workflow Linter extension activated');

  const runCmd = vscode.commands.registerCommand('cadenceLinter.run', () => runLinter());
  context.subscriptions.push(runCmd);

  // Run on save if enabled (debounced)
  vscode.workspace.onDidSaveTextDocument((doc) => {
    const cfg = vscode.workspace.getConfiguration('cadenceLinter');
    if (cfg.get('runOnSave', true) && doc.languageId === 'go') {
      outputChannel.appendLine(`Document saved: ${doc.uri.fsPath} (language=${doc.languageId})`);
      outputChannel.appendLine('runOnSave enabled; scheduling linter run');
      scheduleRunLinter(doc.uri);
    }
  });

  // Run on activation
  scheduleRunLinter();

  // Hover provider: show issue details and callstack (if present) when hovering over an affected line
  const hoverProvider = vscode.languages.registerHoverProvider({ scheme: 'file', language: 'go' }, {
    provideHover(document, position) {
      const filePath = document.uri.fsPath;
      const issues = lastIssues.get(filePath) || [];
      const line = position.line + 1; // issues use 1-based lines
      const hits = issues.filter(i => i.line === line);
      if (hits.length === 0) return undefined;

      // Build markdown content with rule/message and optional output/callstack
  const md = new vscode.MarkdownString();
      for (const iss of hits) {
        md.appendMarkdown(`**${iss.rule}** — ${iss.message}\n\n`);
  const display = `${iss.file}:${iss.line}`;
        try {
          const uri = vscode.Uri.file(iss.file).toString();
          md.appendMarkdown(`[${display}](${uri}#${iss.line})\n\n`);
        } catch (e) {
          md.appendMarkdown(`${display}\n\n`);
        }
      }
  md.isTrusted = true;
      return new vscode.Hover(md);
    }
  });
  context.subscriptions.push(hoverProvider);
}

function scheduleRunLinter(uri?: vscode.Uri) {
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => runLinter(uri), DEBOUNCE_MS);
}

// Helper: read releases manifest bundled with the extension
function readReleasesManifest(): { owner: string; repo: string; tag: string; artifacts: any } | null {
  try {
    if (!extContext) return null;
    const p = path.join(extContext.extensionPath, 'resources', 'releases.json');
    const b = fs.readFileSync(p, 'utf8');
    return JSON.parse(b);
  } catch (e) {
    outputChannel.appendLine('Failed to read releases manifest: ' + String(e));
    return null;
  }
}

function artifactNameForPlatform(manifest: any): string | null {
  const plat = process.platform; // 'win32','darwin','linux'
  const arch = process.arch; // 'x64','arm64', etc.
  const key = plat === 'win32' ? `win32_x64` : (plat === 'darwin' ? (arch === 'arm64' ? 'darwin_arm64' : 'darwin_amd64') : (arch === 'arm64' ? 'linux_arm64' : 'linux_x64'));
  return manifest && manifest.artifacts && manifest.artifacts[key] ? manifest.artifacts[key] : null;
}

async function downloadArtifactTo(extDir: string, url: string, filename: string): Promise<string> {
  const destDir = path.join(extDir, 'bin');
  if (!fs.existsSync(destDir)) fs.mkdirSync(destDir, { recursive: true });
  const dest = path.join(destDir, filename);
  const tmp = dest + '.tmp';
  // Follow HTTP redirects (GitHub release asset URLs redirect to an S3/fastly URL)
  const maxRedirects = 5;
  return new Promise<string>((resolve, reject) => {
    const fetch = (u: string, redirectsLeft: number) => {
      const req = https.get(u, (res) => {
        // Handle redirects
        if (res.statusCode && [301, 302, 303, 307, 308].includes(res.statusCode)) {
          const loc = res.headers.location;
          if (loc && redirectsLeft > 0) {
            // follow redirect
            res.resume(); // discard body
            return fetch(loc, redirectsLeft - 1);
          }
          reject(new Error('Too many redirects or missing Location header'));
          return;
        }

        if (!res.statusCode || res.statusCode < 200 || res.statusCode >= 300) {
          reject(new Error('Download failed with status ' + res.statusCode));
          return;
        }

        const fileStream = fs.createWriteStream(tmp, { mode: 0o755 });
        res.pipe(fileStream);
        fileStream.on('finish', () => {
          fileStream.close();
          try {
            fs.renameSync(tmp, dest);
            if (process.platform !== 'win32') {
              try { fs.chmodSync(dest, 0o755); } catch (e) {}
            }
            resolve(dest);
          } catch (e) { reject(e); }
        });
        fileStream.on('error', (err) => { reject(err); });
      });
      req.on('error', (err) => reject(err));
    };

    fetch(url, maxRedirects);
  });
}

// Fetch JSON from a URL, following redirects (max 5)
async function fetchJson(url: string): Promise<any> {
  const maxRedirects = 5;
  return new Promise<any>((resolve, reject) => {
    const fetch = (u: string, redirectsLeft: number) => {
      const req = https.get(u, (res) => {
        if (res.statusCode && [301,302,303,307,308].includes(res.statusCode)) {
          const loc = res.headers.location;
          if (loc && redirectsLeft > 0) {
            res.resume();
            return fetch(loc, redirectsLeft - 1);
          }
          reject(new Error('Too many redirects or missing Location header'));
          return;
        }
        if (!res.statusCode || res.statusCode < 200 || res.statusCode >= 300) {
          reject(new Error('Request failed with status ' + res.statusCode));
          return;
        }
        let body = '';
        res.on('data', (chunk) => { body += chunk.toString(); });
        res.on('end', () => {
          try { resolve(JSON.parse(body)); } catch (e) { reject(e); }
        });
      });
      req.on('error', (err) => reject(err));
    };
    fetch(url, maxRedirects);
  });
}

function computeFileSha256(filePath: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash('sha256');
    const rs = fs.createReadStream(filePath);
    rs.on('error', (err) => reject(err));
    rs.on('data', (chunk) => hash.update(chunk));
    rs.on('end', () => resolve(hash.digest('hex')));
  });
}

function findInPath(names: string[]): string | null {
  const envPath = process.env.PATH || '';
  const parts = envPath.split(path.delimiter);
  for (const dir of parts) {
    for (const n of names) {
      const candidate = path.join(dir, n);
      if (fs.existsSync(candidate)) return candidate;
      // Windows may have .exe implicit, check with .exe
      if (process.platform === 'win32' && !n.endsWith('.exe')) {
        const c2 = candidate + '.exe';
        if (fs.existsSync(c2)) return c2;
      }
    }
  }
  return null;
}

async function resolveCliPath(cfg: vscode.WorkspaceConfiguration, cwd: string): Promise<string> {
  // 1) user-configured path
  const configured = cfg.get<string>('cliPath');
  if (configured) {
    // If the user provided an explicit path that exists on disk, use it.
    if (fs.existsSync(configured)) return configured;
    // If the configured value looks like a filename or a wrong path, log and continue
    outputChannel.appendLine(`Configured CLI path set but not found: ${configured}; will attempt discovery/download instead`);
  }

  // 2) previously downloaded binary in globalStorage
  if (extContext) {
    const saved = extContext.globalState.get<string>('downloadedCliPath');
    if (saved && fs.existsSync(saved)) return saved;
  }

  // 3) check workspace common locations
  const manifest = readReleasesManifest();
  const artifact = artifactNameForPlatform(manifest);
  const candPaths: string[] = [];
  if (artifact) {
    candPaths.push(path.join(cwd, '.vscode', '.cadence-linter', 'bin', artifact));
    candPaths.push(path.join(cwd, 'bin', artifact));
  }
  for (const p of candPaths) if (fs.existsSync(p)) return p;

  // 4) check PATH for common binary names
  const names = ['cadence-linter', 'cadence-workflow-linter', 'cadence-workflow-linter.exe', 'cadence-linter.exe'];
  const found = findInPath(names);
  if (found) return found;

  // 5) try to download from Releases (manifest provides artifact name)
  if (manifest && artifact) {
    try {
      if (!extContext) throw new Error('extension context not set');
      const owner = manifest.owner;
      const repo = manifest.repo;
      const tag = manifest.tag || 'latest';
      // Try to fetch manifest.json from the Release (so we can verify checksums)
      try {
        const manifestUrl = `https://github.com/${owner}/${repo}/releases/${tag === 'latest' ? 'latest/download' : 'download/' + tag}/manifest.json`;
        outputChannel.appendLine(`Fetching release manifest from ${manifestUrl} ...`);
        const remoteManifest = await fetchJson(manifestUrl);
        const artifactsMap = remoteManifest && remoteManifest.artifacts ? remoteManifest.artifacts : null;
        const expectedSha = artifactsMap ? artifactsMap[artifact] : null;

        // If we previously downloaded a CLI, check if its checksum matches the remote manifest
        if (extContext) {
          const savedPath = extContext.globalState.get<string>('downloadedCliPath');
          const savedSha = extContext.globalState.get<string>('downloadedCliChecksum');
          const savedTag = extContext.globalState.get<string>('downloadedCliTag');
          if (savedPath && fs.existsSync(savedPath) && expectedSha && savedSha === expectedSha && savedTag === (remoteManifest.tag || tag)) {
            outputChannel.appendLine('Using previously-downloaded CLI (checksum matches manifest)');
            return savedPath;
          }
        }

        // Download the artifact and verify checksum
        const url = `https://github.com/${owner}/${repo}/releases/${tag === 'latest' ? 'latest/download' : 'download/' + tag}/${artifact}`;
        outputChannel.appendLine(`Downloading CLI from ${url} ...`);
        const binPath = await downloadArtifactTo(extContext.globalStoragePath, url, artifact);
        if (expectedSha) {
          const realSha = await computeFileSha256(binPath);
          if (realSha !== expectedSha) {
            // delete bad file
            try { fs.unlinkSync(binPath); } catch (e) {}
            throw new Error(`Downloaded file checksum mismatch: expected ${expectedSha} got ${realSha}`);
          }
        } else {
          outputChannel.appendLine('Warning: no expected checksum in manifest; skipping verification');
        }

        // store path, checksum and tag
        try { await extContext.globalState.update('downloadedCliPath', binPath); await extContext.globalState.update('downloadedCliChecksum', expectedSha || ''); await extContext.globalState.update('downloadedCliTag', remoteManifest.tag || tag); } catch (e) {}
        return binPath;
      } catch (e) {
        outputChannel.appendLine('Release manifest fetch/verify failed: ' + String(e));
        // fallback: attempt direct download without manifest
        const url = `https://github.com/${owner}/${repo}/releases/${tag === 'latest' ? 'latest/download' : 'download/' + tag}/${artifact}`;
        outputChannel.appendLine(`Downloading CLI from ${url} ...`);
        const binPath = await downloadArtifactTo(extContext.globalStoragePath, url, artifact);
        try { extContext.globalState.update('downloadedCliPath', binPath); } catch (e) {}
        return binPath;
      }
    } catch (e) {
      outputChannel.appendLine('CLI download failed: ' + String(e));
    }
  }

  // nothing found — do NOT return a non-existent configured path; return a fallback
  return 'cadence-workflow-linter';
}

function resolveRulesPath(cwd: string): string {
  // prefer workspace rules
  const wkRules = path.join(cwd, 'config', 'rules.yaml');
  if (fs.existsSync(wkRules)) return wkRules;
  // fallback: copy extension default rules to temp and return path
  if (extContext) {
    try {
      const src = path.join(extContext.extensionPath, 'resources', 'default-rules.yaml');
      if (fs.existsSync(src)) {
        const dest = path.join(os.tmpdir(), `cadence-linter-rules-${Date.now()}.yaml`);
        fs.copyFileSync(src, dest);
        return dest;
      }
    } catch (e) {
      outputChannel.appendLine('Failed to copy default rules: ' + String(e));
    }
  }
  return wkRules; // last resort (may not exist)
}

async function runLinter(targetUri?: vscode.Uri) {
  const cfg = vscode.workspace.getConfiguration('cadenceLinter');
  const extraArgs = cfg.get<string[]>('args', []);

  const workspaceFolders = vscode.workspace.workspaceFolders;
  if (!workspaceFolders || workspaceFolders.length === 0) {
    vscode.window.showWarningMessage('No workspace folder open');
    return;
  }
  const cwd = workspaceFolders[0].uri.fsPath;

  const args = ['--format', 'json'].concat(extraArgs);
  // resolve CLI path (may download) and rules path before finalizing args
  const cliPath = await resolveCliPath(cfg, cwd);
  const rulesPath = resolveRulesPath(cwd);
  // push rules path
  args.push('--rules', rulesPath);
  if (targetUri) args.push(targetUri.fsPath); else args.push(cwd);

  outputChannel.appendLine(`Running: ${cliPath} ${args.map(a => a.includes(' ') ? '"' + a + '"' : a).join(' ')}`);

  await vscode.window.withProgress({ location: vscode.ProgressLocation.Notification, title: 'Cadence Workflow Linter', cancellable: false }, async (progress) => {
    progress.report({ message: 'Running linter...' });

    return new Promise<void>((resolve) => {
      const spawnAndCollect = (bin: string, a: string[]) => {
        return new Promise<{stdout: string, stderr: string, code: number|null, err?: any}>((res) => {
          const c = spawn(bin, a, { cwd });
          let _stdout = '';
          let _stderr = '';
          c.stdout?.on('data', (chunk) => {
            const s = chunk.toString();
            _stdout += s;
            outputChannel.append(s);
          });
          c.stderr?.on('data', (chunk) => {
            const s = chunk.toString();
            _stderr += s;
            outputChannel.append(s);
          });
          c.on('error', (err) => {
            res({ stdout: _stdout, stderr: _stderr, code: null, err });
          });
          c.on('close', (code) => {
            res({ stdout: _stdout, stderr: _stderr, code });
          });
        });
      };

      // Try configured CLI first; if it's not available, attempt `go run ./cmd/cadence-linter` as a sensible fallback.
      (async () => {
        let result = await spawnAndCollect(cliPath, args);
        if (result.err && (result.err as any).code === 'ENOENT') {
          outputChannel.appendLine(`Configured CLI not found: ${cliPath} — attempting 'go run ./cmd/cadence-linter' fallback`);
          const goArgs = ['run', './cmd/cadence-linter', '--format', 'json', '--rules', rulesPath].concat(extraArgs);
          if (targetUri) goArgs.push(targetUri.fsPath); else goArgs.push(cwd);
          result = await spawnAndCollect('go', goArgs);
        }

        if (result.err && (result.err as any).code !== 'ENOENT') {
          const msg = result.stderr || (result.err && (result.err as any).message) || String(result.err);
          vscode.window.showErrorMessage('cadence-workflow-linter failed: ' + msg);
          outputChannel.appendLine('Error: ' + msg);
          resolve();
          return;
        }

        if (result.code !== 0) {
          outputChannel.appendLine(`cadence-workflow-linter exited with code ${result.code}`);
        }

        try {
          const issues: CliIssue[] = JSON.parse(result.stdout || '[]');
          publishDiagnostics(issues);
        } catch (e) {
          const msg = e && typeof e === 'object' && 'message' in e ? (e as any).message : String(e);
          vscode.window.showErrorMessage('Failed to parse cadence-workflow-linter output: ' + msg);
          outputChannel.appendLine('Failed to parse output: ' + msg);
          outputChannel.appendLine('STDOUT: ' + result.stdout);
          outputChannel.appendLine('STDERR: ' + result.stderr);
        }
        resolve();
      })();
    });
  });
}

function publishDiagnostics(issues: CliIssue[]) {
  diagnosticCollection.clear();
  const workspaceFolders = vscode.workspace.workspaceFolders;
  if (!workspaceFolders) return;
  const cwd = workspaceFolders[0].uri.fsPath;

  const diagMap: Map<string, vscode.Diagnostic[]> = new Map();
  for (const issue of issues) {
    const filePath = path.isAbsolute(issue.file) ? issue.file : path.join(cwd, issue.file);
    // Use a full-line range (ignore column): start at column 0, end at a large column index
    const range = new vscode.Range(issue.line - 1, 0, issue.line - 1, 1000);
    const severity = issue.severity === 'error' ? vscode.DiagnosticSeverity.Error : (issue.severity === 'warning' ? vscode.DiagnosticSeverity.Warning : vscode.DiagnosticSeverity.Information);
    const diag = new vscode.Diagnostic(range, `${issue.rule}: ${issue.message}`, severity);
    diag.source = 'cadence-workflow-linter';

    const arr = diagMap.get(filePath) || [];
    arr.push(diag);
    diagMap.set(filePath, arr);
  }

  // store last issues for hover provider
  lastIssues.clear();
  for (const iss of issues) {
    const fp = path.isAbsolute(iss.file) ? iss.file : path.join(cwd, iss.file);
    const arr = lastIssues.get(fp) || [];
    arr.push(iss);
    lastIssues.set(fp, arr);
  }

  diagMap.forEach((diags, file) => {
    const uri = vscode.Uri.file(file);
    diagnosticCollection.set(uri, diags);
  });
}

export function deactivate() {
  if (diagnosticCollection) {
    diagnosticCollection.dispose();
  }
}
