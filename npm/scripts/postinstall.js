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

function releaseTag(version) {
  if (!version) {
    fail("invalid package version");
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

function downloadBuffer(url) {
  return new Promise((resolve, reject) => {
    const req = https.get(url, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        resolve(downloadBuffer(res.headers.location));
        return;
      }
      if (res.statusCode !== 200) {
        reject(new Error(`download failed (${res.statusCode}) for ${url}`));
        return;
      }
      const chunks = [];
      res.on("data", (chunk) => chunks.push(chunk));
      res.on("end", () => resolve(Buffer.concat(chunks)));
    });

    req.on("error", (err) => reject(err));
  });
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

async function main() {
  if (process.env.ASM_SKIP_POSTINSTALL === "1") {
    console.log("[agent-manager] skipping postinstall download (ASM_SKIP_POSTINSTALL=1)");
    return;
  }

  const tag = releaseTag(pkg.version);
  const repo = process.env.ASM_RELEASE_REPO || pkg.config.releaseRepo;
  if (!repo || repo.includes("OWNER/REPO")) {
    fail("release repository is not configured");
  }

  const { assetName, targetName } = platformAsset();
  const baseURL =
    process.env.ASM_RELEASE_BASE_URL || `https://github.com/${repo}/releases/download/${tag}`;
  const assetURL = `${baseURL}/${assetName}`;
  const checksumsURL = `${baseURL}/checksums.txt`;

  const [binaryData, checksumsData] = await Promise.all([
    downloadBuffer(assetURL),
    downloadBuffer(checksumsURL),
  ]);

  const expected = checksumFromFile(checksumsData.toString("utf8"), assetName);
  if (!expected) {
    fail(`checksum entry not found for ${assetName}`);
  }

  const actual = crypto.createHash("sha256").update(binaryData).digest("hex");
  if (actual !== expected) {
    fail(`checksum mismatch for ${assetName}`);
  }

  const targetDir = path.join(__dirname, "..", "bin");
  const targetPath = path.join(targetDir, targetName);
  fs.mkdirSync(targetDir, { recursive: true });
  fs.writeFileSync(targetPath, binaryData);
  if (os.platform() !== "win32") {
    fs.chmodSync(targetPath, 0o755);
  }

  console.log(`[agent-manager] installed ${assetName}`);
}

main().catch((err) => fail(err.message));
