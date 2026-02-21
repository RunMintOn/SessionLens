#!/usr/bin/env node
"use strict";

const crypto = require("node:crypto");
const fs = require("node:fs");
const https = require("node:https");
const os = require("node:os");
const path = require("node:path");

const pkg = require("../package.json");

function fail(message) {
  console.error(`[agent-manager] ${message}`);
  process.exit(1);
}

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
    if (match?.groups?.repo) {
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

  const fromConfig = pkg?.config?.releaseRepo;
  if (fromConfig && !fromConfig.includes("OWNER/REPO")) {
    return fromConfig;
  }

  const fromRepoURL = parseRepoFromURL(pkg?.repository?.url);
  if (fromRepoURL) {
    return fromRepoURL;
  }

  return "";
}

function releaseTag(version) {
  if (!version) {
    return "";
  }
  return version.startsWith("v") ? version : `v${version}`;
}

function platformAsset() {
  if (os.arch() !== "x64") {
    fail(`unsupported architecture: ${os.arch()} (supported: x64)`);
  }

  switch (os.platform()) {
    case "linux":
      return {
        assetName: "agent-manager-linux-amd64",
        targetName: "agent-manager",
      };
    case "win32":
      return {
        assetName: "agent-manager-windows-amd64.exe",
        targetName: "agent-manager.exe",
      };
    default:
      fail(`unsupported platform: ${os.platform()} (supported: linux, win32)`);
  }
}

function downloadBuffer(url, accept = "application/octet-stream") {
  return new Promise((resolve, reject) => {
    const req = https.get(
      url,
      {
        headers: {
          "User-Agent": "agent-session-manager-installer",
          Accept: accept,
        },
      },
      (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          resolve(downloadBuffer(res.headers.location, accept));
          return;
        }
        if (res.statusCode !== 200) {
          reject(new Error(`download failed (${res.statusCode}) for ${url}`));
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
  const [binaryData, checksumsData] = await Promise.all([downloadBuffer(assetURL), downloadBuffer(checksumsURL)]);
  return { binaryData, checksumsData };
}

async function main() {
  if (process.env.ASM_SKIP_POSTINSTALL === "1") {
    console.log("[agent-manager] skipping postinstall download (ASM_SKIP_POSTINSTALL=1)");
    return;
  }

  const repo = inferReleaseRepo();
  if (!repo) {
    fail("release repository is not configured (set ASM_RELEASE_REPO)");
  }

  const { assetName, targetName } = platformAsset();

  let tag = process.env.ASM_RELEASE_TAG || releaseTag(pkg.version);
  let downloaded;
  let firstErr;
  if (tag) {
    try {
      downloaded = await downloadReleaseAsset(repo, tag, assetName);
    } catch (err) {
      firstErr = err;
    }
  }

  if (!downloaded) {
    try {
      const latestTag = await getLatestReleaseTag(repo);
      if (!latestTag) {
        throw new Error("latest release tag not found");
      }
      tag = latestTag;
      downloaded = await downloadReleaseAsset(repo, tag, assetName);
    } catch (latestErr) {
      if (firstErr) {
        fail(`${firstErr.message}; fallback to latest release failed: ${latestErr.message}`);
      }
      fail(`failed to download release asset: ${latestErr.message}`);
    }
  }

  const expected = checksumFromFile(downloaded.checksumsData.toString("utf8"), assetName);
  if (!expected) {
    fail(`checksum entry not found for ${assetName}`);
  }

  const actual = crypto.createHash("sha256").update(downloaded.binaryData).digest("hex");
  if (actual !== expected) {
    fail(`checksum mismatch for ${assetName}`);
  }

  const targetDir = path.join(__dirname, "..", "bin");
  const targetPath = path.join(targetDir, targetName);
  fs.mkdirSync(targetDir, { recursive: true });
  fs.writeFileSync(targetPath, downloaded.binaryData);
  if (os.platform() !== "win32") {
    fs.chmodSync(targetPath, 0o755);
  }

  console.log(`[agent-manager] installed ${assetName} from ${repo}@${tag}`);
}

main().catch((err) => fail(err.message));
