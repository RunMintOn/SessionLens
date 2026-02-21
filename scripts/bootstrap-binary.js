#!/usr/bin/env node
"use strict";

const path = require("node:path");

const { baseBinaryName, ensureBinary, formatBootstrapError } = require("./lib/install-binary");

async function main() {
  const binaryName = process.platform === "win32" ? `${baseBinaryName()}.exe` : baseBinaryName();
  const binaryPath = path.join(__dirname, "..", "bin", binaryName);

  try {
    const result = await ensureBinary(binaryPath);
    if (result.installed) {
      console.log(`[agent-manager] installed ${result.assetName} from ${result.repo}@${result.tag}`);
      return;
    }
    console.log("[agent-manager] binary already present.");
  } catch (err) {
    console.error(formatBootstrapError(err));
    process.exit(1);
  }
}

main();
