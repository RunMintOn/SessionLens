#!/usr/bin/env node
"use strict";

const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { spawn } = require("node:child_process");

const binaryName = os.platform() === "win32" ? "agent-manager.exe" : "agent-manager";
const binaryPath = path.join(__dirname, binaryName);

if (!fs.existsSync(binaryPath)) {
  console.error("[agent-manager] binary is missing.");
  console.error("[agent-manager] reinstall with: npm i -g agent-session-manager");
  process.exit(1);
}

if (os.platform() !== "win32") {
  try {
    fs.chmodSync(binaryPath, 0o755);
  } catch (err) {
    // Non-fatal; spawn will surface permission errors if any.
  }
}

const child = spawn(binaryPath, process.argv.slice(2), { stdio: "inherit" });

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
