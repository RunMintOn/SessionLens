"""Session scanner package - unified interface for all platform scanners."""

from typing import List, Optional

from models import Session
from scanner.opencode_scanner import OpenCodeScanner
from scanner.claude_scanner import ClaudeCodeScanner
from scanner.qwen_scanner import QwenCodeScanner


def scan_all(project_path: Optional[str] = None) -> List[Session]:
    """
    Scan all platforms for sessions.

    Args:
        project_path: Optional specific project to filter by

    Returns:
        Combined list of sessions from all platforms, sorted by last_updated
    """
    all_sessions = []

    scanners = [
        OpenCodeScanner(),
        ClaudeCodeScanner(),
        QwenCodeScanner(),
    ]

    for scanner in scanners:
        try:
            sessions = scanner.scan(project_path)
            all_sessions.extend(sessions)
        except Exception:
            pass

    all_sessions.sort(key=lambda s: s.last_updated, reverse=True)
    return all_sessions


def scan_opencode(project_path: Optional[str] = None) -> List[Session]:
    """Scan OpenCode sessions."""
    return OpenCodeScanner().scan(project_path)


def scan_claude(project_path: Optional[str] = None) -> List[Session]:
    """Scan Claude Code sessions."""
    return ClaudeCodeScanner().scan(project_path)


def scan_qwen(project_path: Optional[str] = None) -> List[Session]:
    """Scan Qwen Code sessions."""
    return QwenCodeScanner().scan(project_path)


__all__ = [
    "Session",
    "OpenCodeScanner",
    "ClaudeCodeScanner",
    "QwenCodeScanner",
    "scan_all",
    "scan_opencode",
    "scan_claude",
    "scan_qwen",
]