"use strict";

const crypto = require("node:crypto");
const fs = require("node:fs");
const https = require("node:https");
const os = require("node:os");
const path = require("node:path");

const pkg = require("../../package.json");

const DEFAULT_LOCK_TIMEOUT_MS = 120000;
const DEFAULT_LOCK_POLL_MS = 200;

function parseRepoFromURL(input) {
  if (!input) {
    return "";
  }

  const source = String(input);
  const patterns = [
    /github:(?<repo>[^#\s]+)/,
    /github\.com[:/](?<repo>[^#\s]+?)(?:\.git)?(?:#|$)/,
  ];
  for (const pattern of patterns) {
    const match = source.match(pattern);
    if (match && match.groups && match.groups.repo) {
      return match.groups.repo.replace(/\.git$/, "");
    }
  }
  return "";
}

function inferReleaseRepo() {
  if (process.env.ASM_RELEASE_REPO) {
    return process.env.ASM_RELEASE_REPO;
  }

  const fromResolved = parseRepoFromURL(process.env.npm_package_resolved);
  if (fromResolved) {
    return fromResolved;
  }

  const fromFrom = parseRepoFromURL(process.env.npm_package_from);
  if (fromFrom) {
    return fromFrom;
  }

  const fromConfig = pkg && pkg.config ? pkg.config.releaseRepo : "";
  if (fromConfig && !String(fromConfig).includes("OWNER/REPO")) {
    return fromConfig;
  }

  const fromRepoURL = parseRepoFromURL(pkg && pkg.repository ? pkg.repository.url : "");
  if (fromRepoURL) {
    return fromRepoURL;
  }

  return "";
}

function releaseTag(version) {
  if (!version) {
    return "";
  }
  return String(version).startsWith("v") ? String(version) : `v${version}`;
}

function baseBinaryName() {
  if (process.env.ASM_BINARY_NAME) {
    return process.env.ASM_BINARY_NAME;
  }
  if (pkg && pkg.config && pkg.config.binaryName) {
    return pkg.config.binaryName;
  }
  return "agent-manager";
}

function platformAsset() {
  if (os.arch() !== "x64") {
    throw new Error(`unsupported architecture: ${os.arch()} (supported: x64)`);
  }

  const name = baseBinaryName();
  switch (os.platform()) {
    case "linux":
      return {
        assetName: `${name}-linux-amd64`,
        targetName: name,
      };
    case "win32":
      return {
        assetName: `${name}-windows-amd64.exe`,
        targetName: `${name}.exe`,
      };
    default:
      throw new Error(`unsupported platform: ${os.platform()} (supported: linux, win32)`);
  }
}

function downloadBuffer(url, accept) {
  return new Promise((resolve, reject) => {
    const req = https.get(
      url,
      {
        headers: {
          "User-Agent": "agent-session-manager-installer",
          Accept: accept || "application/octet-stream",
        },
      },
      (res) => {
        const statusCode = res.statusCode || 0;
        if (statusCode >= 300 && statusCode < 400 && res.headers.location) {
          const redirectURL = new URL(res.headers.location, url).toString();
          resolve(downloadBuffer(redirectURL, accept));
          return;
        }

        if (statusCode !== 200) {
          reject(new Error(`download failed (${statusCode}) for ${url}`));
          return;
        }

        const chunks = [];
        res.on("data", (chunk) => chunks.push(chunk));
        res.on("end", () => resolve(Buffer.concat(chunks)));
      },
    );

    req.on("error", (err) => reject(err));
  });
}

async function getLatestReleaseTag(repo) {
  const data = await downloadBuffer(
    `https://api.github.com/repos/${repo}/releases/latest`,
    "application/vnd.github+json",
  );
  const parsed = JSON.parse(data.toString("utf8"));
  return String(parsed.tag_name || "").trim();
}

function checksumFromFile(checksumsText, assetName) {
  const line = checksumsText
    .split(/\r?\n/)
    .map((s) => s.trim())
    .find((s) => s.endsWith(`  ${assetName}`) || s.endsWith(` *${assetName}`));

  if (!line) {
    return "";
  }
  return line.split(/\s+/)[0];
}

async function downloadReleaseAsset(repo, tag, assetName) {
  const baseURL = process.env.ASM_RELEASE_BASE_URL || `https://github.com/${repo}/releases/download/${tag}`;
  const assetURL = `${baseURL}/${assetName}`;
  const checksumsURL = `${baseURL}/checksums.txt`;
  const data = await Promise.all([downloadBuffer(assetURL), downloadBuffer(checksumsURL)]);
  return {
    binaryData: data[0],
    checksumsData: data[1],
  };
}

function ensureExecutable(pathName) {
  if (os.platform() === "win32") {
    return;
  }
  try {
    fs.chmodSync(pathName, 0o755);
  } catch (err) {
    // Non-fatal. Launch path will expose permission failures if this fails.
  }
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function numericEnv(name, fallback) {
  const raw = process.env[name];
  if (!raw) {
    return fallback;
  }
  const parsed = Number.parseInt(raw, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback;
  }
  return parsed;
}

async function withLock(lockPath, fn) {
  const timeoutMs = numericEnv("ASM_BOOTSTRAP_LOCK_TIMEOUT_MS", DEFAULT_LOCK_TIMEOUT_MS);
  const pollMs = numericEnv("ASM_BOOTSTRAP_LOCK_POLL_MS", DEFAULT_LOCK_POLL_MS);
  const started = Date.now();

  while (true) {
    let fd;
    try {
      fd = fs.openSync(lockPath, "wx", 0o644);
    } catch (err) {
      if (err && err.code === "EEXIST") {
        if (Date.now() - started > timeoutMs) {
          throw new Error(`timed out waiting for bootstrap lock: ${lockPath}`);
        }
        await sleep(pollMs);
        continue;
      }
      throw err;
    }

    try {
      fs.writeFileSync(fd, `${process.pid}\n`, "utf8");
      return await fn();
    } finally {
      try {
        fs.closeSync(fd);
      } catch (err) {
        // Ignore close failures; lock cleanup is best effort.
      }
      fs.rmSync(lockPath, { force: true });
    }
  }
}

function binaryExists(binaryPath) {
  try {
    return fs.statSync(binaryPath).isFile();
  } catch (err) {
    return false;
  }
}

async function installBinary(targetPath) {
  const repo = inferReleaseRepo();
  if (!repo) {
    throw new Error("release repository is not configured (set ASM_RELEASE_REPO)");
  }

  const asset = platformAsset();
  const explicitTag = process.env.ASM_RELEASE_TAG;
  let tag = explicitTag || releaseTag(pkg.version);
  let downloaded;
  let firstErr;

  if (tag) {
    try {
      downloaded = await downloadReleaseAsset(repo, tag, asset.assetName);
    } catch (err) {
      firstErr = err;
    }
  }

  if (!downloaded) {
    const allowLatestFallback = process.env.ASM_ALLOW_LATEST_FALLBACK === "1";
    if (!allowLatestFallback) {
      if (firstErr) {
        throw new Error(
          `${firstErr.message}; latest fallback disabled to avoid version drift (set ASM_ALLOW_LATEST_FALLBACK=1 to enable)`,
        );
      }
      throw new Error("release asset download failed and latest fallback is disabled");
    }

    try {
      const latestTag = await getLatestReleaseTag(repo);
      if (!latestTag) {
        throw new Error("latest release tag not found");
      }
      tag = latestTag;
      downloaded = await downloadReleaseAsset(repo, tag, asset.assetName);
    } catch (latestErr) {
      if (firstErr) {
        throw new Error(`${firstErr.message}; fallback to latest release failed: ${latestErr.message}`);
      }
      throw new Error(`failed to download release asset: ${latestErr.message}`);
    }
  }

  const expected = checksumFromFile(downloaded.checksumsData.toString("utf8"), asset.assetName);
  if (!expected) {
    throw new Error(`checksum entry not found for ${asset.assetName}`);
  }

  const actual = crypto.createHash("sha256").update(downloaded.binaryData).digest("hex");
  if (actual !== expected) {
    throw new Error(`checksum mismatch for ${asset.assetName}`);
  }

  const targetDir = path.dirname(targetPath);
  fs.mkdirSync(targetDir, { recursive: true });

  const tempPath = `${targetPath}.tmp-${process.pid}-${Date.now()}`;
  fs.writeFileSync(tempPath, downloaded.binaryData);
  ensureExecutable(tempPath);
  fs.renameSync(tempPath, targetPath);
  ensureExecutable(targetPath);

  return {
    repo,
    tag,
    assetName: asset.assetName,
    targetPath,
  };
}

async function ensureBinary(binaryPath) {
  if (binaryExists(binaryPath)) {
    ensureExecutable(binaryPath);
    return { installed: false, targetPath: binaryPath };
  }

  if (process.env.ASM_SKIP_BOOTSTRAP === "1") {
    throw new Error("binary is missing and bootstrap is disabled (ASM_SKIP_BOOTSTRAP=1)");
  }

  const lockPath = `${binaryPath}.bootstrap.lock`;
  return withLock(lockPath, async () => {
    if (binaryExists(binaryPath)) {
      ensureExecutable(binaryPath);
      return { installed: false, targetPath: binaryPath };
    }
    const result = await installBinary(binaryPath);
    return {
      installed: true,
      repo: result.repo,
      tag: result.tag,
      assetName: result.assetName,
      targetPath: result.targetPath,
    };
  });
}

function formatBootstrapError(err) {
  const message = err && err.message ? err.message : String(err);
  const suggestedTag = releaseTag(pkg && pkg.version ? pkg.version : "0.1.3");
  return [
    `[agent-manager] bootstrap failed: ${message}`,
    "[agent-manager] Try:",
    "[agent-manager]   1) check network access to github.com",
    `[agent-manager]   2) pin a release tag, e.g. ASM_RELEASE_TAG=${suggestedTag} agent-manager`,
    "[agent-manager]   3) override mirror URL, e.g. ASM_RELEASE_BASE_URL=<url> agent-manager",
  ].join("\n");
}

module.exports = {
  baseBinaryName,
  ensureBinary,
  formatBootstrapError,
};
