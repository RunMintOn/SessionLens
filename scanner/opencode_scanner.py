"""OpenCode session scanner - reads SQLite database."""

import os
import sqlite3
from pathlib import Path
from typing import List, Optional

from models import Session


class OpenCodeScanner:
    """Scanner for OpenCode sessions from SQLite database."""

    DB_PATH = Path.home() / ".local" / "share" / "opencode" / "opencode.db"

    def __init__(self, db_path: Optional[Path] = None):
        """Initialize scanner with optional custom db path."""
        self.db_path = db_path or self.DB_PATH

    def scan(self, project_path: Optional[str] = None) -> List[Session]:
        """
        Scan for OpenCode sessions.

        Args:
            project_path: Optional specific project to filter by

        Returns:
            List of Session objects
        """
        sessions = []

        if not self.db_path.exists():
            return sessions

        try:
            conn = sqlite3.connect(str(self.db_path))
            cursor = conn.cursor()

            # Query sessions table - adjust column names based on actual schema
            cursor.execute("""
                SELECT id, title, directory, time_updated
                FROM session
                WHERE parent_id IS NULL
                ORDER BY time_updated DESC
            """)

            for row in cursor.fetchall():
                session_id, title, directory, time_updated = row

                # Filter by project path if specified
                if project_path and directory != project_path:
                    continue

                sessions.append(Session(
                    id=str(session_id),
                    title=title or "Untitled Session",
                    source_tool="opencode",
                    project_path=directory or "",
                    last_updated=int(time_updated) if time_updated else 0,
                ))

            conn.close()

        except (sqlite3.Error, OSError):
            # Gracefully skip on error
            pass

        return sessions

    def get_projects(self) -> List[str]:
        """Get list of unique project directories from OpenCode sessions."""
        projects = []

        if not self.db_path.exists():
            return projects

        try:
            conn = sqlite3.connect(str(self.db_path))
            cursor = conn.cursor()

            cursor.execute("""
                SELECT DISTINCT directory
                FROM session
                WHERE parent_id IS NULL
                WHERE directory IS NOT NULL AND directory != ''
                ORDER BY directory
            """)

            projects = [row[0] for row in cursor.fetchall()]

            conn.close()

        except (sqlite3.Error, OSError):
            pass

        return projects
