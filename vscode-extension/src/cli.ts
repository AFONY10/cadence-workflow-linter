import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';
import * as https from 'https';
import * as crypto from 'crypto';
import { extContext, outputChannel } from './context';

// Read bundled releases.json
export function readReleasesManifest(): { owner: string; repo: string; tag: string; artifacts: any } | null {
  try {
    if (!extContext) return null;
    const p = path.join(extContext.extensionPath, 'resources', 'releases.json');
    const b = fs.readFileSync(p, 'utf8');
    return JSON.parse(b);
  } catch (e) {
    try { outputChannel.appendLine('Failed to read releases manifest: ' + String(e)); } catch (e) {}
    return null;
  }
}

export function artifactNameForPlatform(manifest: any): string | null {
  const plat = process.platform;
  const arch = process.arch;
  const key = plat === 'win32' ? `win32_x64` : (plat === 'darwin' ? (arch === 'arm64' ? 'darwin_arm64' : 'darwin_amd64') : (arch === 'arm64' ? 'linux_arm64' : 'linux_x64'));
  return manifest && manifest.artifacts && manifest.artifacts[key] ? manifest.artifacts[key] : null;
}

// Follow redirects and download to ext storage
export async function downloadArtifactTo(extDir: string, url: string, filename: string): Promise<string> {
  const destDir = path.join(extDir, 'bin');
  if (!fs.existsSync(destDir)) fs.mkdirSync(destDir, { recursive: true });
  const dest = path.join(destDir, filename);
  const tmp = dest + '.tmp';
  const maxRedirects = 5;
  return new Promise<string>((resolve, reject) => {
    const fetch = (u: string, redirectsLeft: number) => {
      const req = https.get(u, (res) => {
        if (res.statusCode && [301, 302, 303, 307, 308].includes(res.statusCode)) {
          const loc = res.headers.location;
          if (loc && redirectsLeft > 0) { res.resume(); return fetch(loc, redirectsLeft - 1); }
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

export async function fetchJson(url: string): Promise<any> {
  const maxRedirects = 5;
  return new Promise<any>((resolve, reject) => {
    const fetch = (u: string, redirectsLeft: number) => {
      const req = https.get(u, (res) => {
        if (res.statusCode && [301,302,303,307,308].includes(res.statusCode)) {
          const loc = res.headers.location;
          if (loc && redirectsLeft > 0) { res.resume(); return fetch(loc, redirectsLeft - 1); }
          reject(new Error('Too many redirects or missing Location header')); return;
        }
        if (!res.statusCode || res.statusCode < 200 || res.statusCode >= 300) {
          reject(new Error('Request failed with status ' + res.statusCode)); return;
        }
        let body = '';
        res.on('data', (chunk) => { body += chunk.toString(); });
        res.on('end', () => { try { resolve(JSON.parse(body)); } catch (e) { reject(e); } });
      });
      req.on('error', (err) => reject(err));
    };
    fetch(url, maxRedirects);
  });
}

export function computeFileSha256(filePath: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash('sha256');
    const rs = fs.createReadStream(filePath);
    rs.on('error', (err) => reject(err));
    rs.on('data', (chunk) => hash.update(chunk));
    rs.on('end', () => resolve(hash.digest('hex')));
  });
}

export function findInPath(names: string[]): string | null {
  const envPath = process.env.PATH || '';
  const parts = envPath.split(path.delimiter);
  for (const dir of parts) {
    for (const n of names) {
      const candidate = path.join(dir, n);
      if (fs.existsSync(candidate)) return candidate;
      if (process.platform === 'win32' && !n.endsWith('.exe')) {
        const c2 = candidate + '.exe'; if (fs.existsSync(c2)) return c2;
      }
    }
  }
  return null;
}

export function resolveRulesPath(cwd: string): string {
  const wkRules = path.join(cwd, 'config', 'rules.yaml');
  if (fs.existsSync(wkRules)) return wkRules;
  if (extContext) {
    try {
      const src = path.join(extContext.extensionPath, 'resources', 'default-rules.yaml');
      if (fs.existsSync(src)) {
        const dest = path.join(os.tmpdir(), `cadence-linter-rules-${Date.now()}.yaml`);
        fs.copyFileSync(src, dest);
        return dest;
      }
    } catch (e) {
      try { outputChannel.appendLine('Failed to copy default rules: ' + String(e)); } catch (e) {}
    }
  }
  return wkRules;
}

export async function resolveCliPath(cfg: vscode.WorkspaceConfiguration, cwd: string): Promise<string> {
  const configured = cfg.get<string>('cliPath');
  if (configured) {
    if (fs.existsSync(configured)) return configured;
    try { outputChannel.appendLine(`Configured CLI path set but not found: ${configured}; will attempt discovery/download instead`); } catch (e) {}
  }

  if (extContext) {
    const saved = extContext.globalState.get<string>('downloadedCliPath');
    if (saved && fs.existsSync(saved)) return saved;
  }

  const manifest = readReleasesManifest();
  const artifact = artifactNameForPlatform(manifest);
  const candPaths: string[] = [];
  if (artifact) { candPaths.push(path.join(cwd, '.vscode', '.cadence-linter', 'bin', artifact)); candPaths.push(path.join(cwd, 'bin', artifact)); }
  for (const p of candPaths) if (fs.existsSync(p)) return p;

  const names = ['cadence-linter', 'cadence-workflow-linter', 'cadence-workflow-linter.exe', 'cadence-linter.exe'];
  const found = findInPath(names);
  if (found) return found;

  if (manifest && artifact) {
    try {
      if (!extContext) throw new Error('extension context not set');
      const owner = manifest.owner; const repo = manifest.repo; const tag = manifest.tag || 'latest';
      try {
        const manifestUrl = `https://github.com/${owner}/${repo}/releases/${tag === 'latest' ? 'latest/download' : 'download/' + tag}/manifest.json`;
        try { outputChannel.appendLine(`Fetching release manifest from ${manifestUrl} ...`); } catch (e) {}
        const remoteManifest = await fetchJson(manifestUrl);
        const artifactsMap = remoteManifest && remoteManifest.artifacts ? remoteManifest.artifacts : null;
        const expectedSha = artifactsMap ? artifactsMap[artifact] : null;

        if (extContext) {
          const savedPath = extContext.globalState.get<string>('downloadedCliPath');
          const savedSha = extContext.globalState.get<string>('downloadedCliChecksum');
          const savedTag = extContext.globalState.get<string>('downloadedCliTag');
          if (savedPath && fs.existsSync(savedPath) && expectedSha && savedSha === expectedSha && savedTag === (remoteManifest.tag || tag)) {
            try { outputChannel.appendLine('Using previously-downloaded CLI (checksum matches manifest)'); } catch (e) {}
            return savedPath;
          }
        }

        const url = `https://github.com/${owner}/${repo}/releases/${tag === 'latest' ? 'latest/download' : 'download/' + tag}/${artifact}`;
        try { outputChannel.appendLine(`Downloading CLI from ${url} ...`); } catch (e) {}
        const binPath = await downloadArtifactTo(extContext!.globalStoragePath, url, artifact);
        if (expectedSha) {
          const realSha = await computeFileSha256(binPath);
          if (realSha !== expectedSha) { try { fs.unlinkSync(binPath); } catch (e) {} ; throw new Error(`Downloaded file checksum mismatch: expected ${expectedSha} got ${realSha}`); }
        } else { try { outputChannel.appendLine('Warning: no expected checksum in manifest; skipping verification'); } catch (e) {} }

        try { await extContext!.globalState.update('downloadedCliPath', binPath); await extContext!.globalState.update('downloadedCliChecksum', expectedSha || ''); await extContext!.globalState.update('downloadedCliTag', remoteManifest.tag || tag); } catch (e) {}
        return binPath;
      } catch (e) {
        try { outputChannel.appendLine('Release manifest fetch/verify failed: ' + String(e)); } catch (e) {}
        const url = `https://github.com/${owner}/${repo}/releases/${tag === 'latest' ? 'latest/download' : 'download/' + tag}/${artifact}`;
        try { outputChannel.appendLine(`Downloading CLI from ${url} ...`); } catch (e) {}
        const binPath = await downloadArtifactTo(extContext!.globalStoragePath, url, artifact);
        try { extContext!.globalState.update('downloadedCliPath', binPath); } catch (e) {}
        return binPath;
      }
    } catch (e) {
      try { outputChannel.appendLine('CLI download failed: ' + String(e)); } catch (e) {}
    }
  }

  return 'cadence-workflow-linter';
}
