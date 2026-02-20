"""Qwen Code session scanner - reads JSONL files."""

import json
from pathlib import Path
from typing import List, Optional

from models import Session


class QwenCodeScanner:
    """Scanner for Qwen Code sessions from JSONL files."""

    BASE_PATH = Path.home() / ".qwen" / "projects"

    def __init__(self, base_path: Optional[Path] = None):
        """Initialize scanner with optional custom base path."""
        self.base_path = base_path or self.BASE_PATH

    def scan(self, project_path: Optional[str] = None) -> List[Session]:
        """
        Scan for Qwen Code sessions.

        Args:
            project_path: Optional specific project to filter by

        Returns:
            List of Session objects
        """
        sessions = []

        if not self.base_path.exists():
            return sessions

        if project_path:
            chats_dir = self.base_path / f"-{project_path}" / "chats"
            if chats_dir.exists():
                sessions.extend(self._scan_chats_dir(chats_dir, str(chats_dir.parent)))
        else:
            for proj_dir in self.base_path.iterdir():
                if not proj_dir.is_dir():
                    continue
                chats_dir = proj_dir / "chats"
                if chats_dir.exists():
                    sessions.extend(self._scan_chats_dir(chats_dir, str(proj_dir)))

        sessions.sort(key=lambda s: s.last_updated, reverse=True)
        return sessions

    def _scan_chats_dir(self, chats_dir: Path, project_dir: str) -> List[Session]:
        """Scan a chats directory for Qwen session files."""
        sessions = []

        for jsonl_file in chats_dir.glob("*.jsonl"):
            session = self._parse_session(jsonl_file, project_dir)
            if session:
                sessions.append(session)

        return sessions

    def _parse_session(self, jsonl_path: Path, project_dir: str) -> Optional[Session]:
        """Parse a single Qwen JSONL file."""
        try:
            with open(jsonl_path, "r", encoding="utf-8") as f:
                first_user_message = None
                last_updated = 0

                for line in f:
                    if not line.strip():
                        continue

                    try:
                        data = json.loads(line)
                    except json.JSONDecodeError:
                        continue

                    msg_type = data.get("type")
                    if msg_type == "user" and first_user_message is None:
                        message = data.get("message", {})
                        parts = message.get("parts", [])
                        for part in parts:
                            if isinstance(part, dict) and "text" in part:
                                first_user_message = part["text"][:100]
                                break

                    ts = data.get("timestamp") or data.get("createdAt")
                    if ts:
                        try:
                            ts_int = int(ts)
                            if ts_int > last_updated:
                                last_updated = ts_int
                        except (ValueError, TypeError):
                            pass

                if not first_user_message:
                    first_user_message = jsonl_path.stem

                session_id = jsonl_path.stem
                return Session(
                    id=session_id,
                    title=first_user_message,
                    source_tool="qwen",
                    project_path=project_dir,
                    last_updated=last_updated,
                )

        except (OSError, IOError):
            pass

        return None
