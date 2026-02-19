"""Claude Code session scanner - reads JSONL files."""

import json
from pathlib import Path
from typing import List, Optional

from models import Session


class ClaudeCodeScanner:
    """Scanner for Claude Code sessions from JSONL files."""

    BASE_PATH = Path.home() / ".claude" / "projects"

    def __init__(self, base_path: Optional[Path] = None):
        """Initialize scanner with optional custom base path."""
        self.base_path = base_path or self.BASE_PATH

    def scan(self, project_path: Optional[str] = None) -> List[Session]:
        """
        Scan for Claude Code sessions.

        Args:
            project_path: Optional specific project to filter by

        Returns:
            List of Session objects
        """
        sessions = []

        if not self.base_path.exists():
            return sessions

        project_dirs = [self.base_path / project_path] if project_path else self.base_path.iterdir()

        for proj_dir in project_dirs:
            if not proj_dir.is_dir():
                continue

            for jsonl_file in proj_dir.glob("*.jsonl"):
                session = self._parse_session(jsonl_file, str(proj_dir))
                if session:
                    sessions.append(session)

        sessions.sort(key=lambda s: s.last_updated, reverse=True)
        return sessions

    def _parse_session(self, jsonl_path: Path, project_dir: str) -> Optional[Session]:
        """Parse a single Claude JSONL file."""
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
                    if msg_type == "user":
                        if first_user_message is None:
                            msg_content = data.get("message", {}).get("content", "")
                            if isinstance(msg_content, str):
                                first_user_message = msg_content[:100]
                            elif isinstance(msg_content, list):
                                for part in msg_content:
                                    if isinstance(part, dict) and part.get("type") == "text":
                                        first_user_message = part.get("text", "")[:100]
                                        break

                    ts = data.get("timestamp")
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
                    source_tool="claude",
                    project_path=project_dir,
                    last_updated=last_updated,
                )

        except (OSError, IOError):
            pass

        return None
