#!/usr/bin/env node
"use strict";

const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { spawn } = require("node:child_process");

const { baseBinaryName, ensureBinary, formatBootstrapError } = require("../scripts/lib/install-binary");

function binaryPath() {
  const name = os.platform() === "win32" ? `${baseBinaryName()}.exe` : baseBinaryName();
  return path.join(__dirname, name);
}

function launch(binaryPathName) {
  if (os.platform() !== "win32") {
    try {
      fs.chmodSync(binaryPathName, 0o755);
    } catch (err) {
      // Non-fatal; spawn will surface permission errors if any.
    }
  }

  const child = spawn(binaryPathName, process.argv.slice(2), { stdio: "inherit" });

  child.on("error", (err) => {
    console.error(`[agent-manager] failed to launch binary: ${err.message}`);
    process.exit(1);
  });

  child.on("exit", (code, signal) => {
    if (signal) {
      process.kill(process.pid, signal);
      return;
    }
    process.exit(code ?? 1);
  });
}

async function main() {
  const entryBinaryPath = binaryPath();
  if (!fs.existsSync(entryBinaryPath)) {
    console.log("[agent-manager] first run bootstrap: downloading binary...");
    try {
      const result = await ensureBinary(entryBinaryPath);
      if (result.installed) {
        console.log(`[agent-manager] installed ${result.assetName} from ${result.repo}@${result.tag}`);
      }
    } catch (err) {
      console.error(formatBootstrapError(err));
      process.exit(1);
    }
  }

  launch(entryBinaryPath);
}

main();
