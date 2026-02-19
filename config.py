"""Configuration management for agent session manager."""

import os
import sqlite3
from pathlib import Path
from typing import Optional


class Config:
    """Configuration manager for agent session manager."""

    DEFAULT_APP_DIR = ".agent-session-manager"
    FAVORITES_DB = "favorites.db"

    def __init__(self, data_dir: Optional[str] = None):
        """Initialize configuration.

        Args:
            data_dir: Custom data directory. If None, uses ~/.agent-session-manager/
        """
        if data_dir:
            self._data_dir = Path(data_dir)
        else:
            self._data_dir = Path(os.path.expanduser("~/.agent-session-manager"))

        # Ensure data directory exists
        self._data_dir.mkdir(parents=True, exist_ok=True)

    @property
    def data_dir(self) -> Path:
        """Get the data directory path.

        Returns:
            Path to the data directory.
        """
        return self._data_dir

    @property
    def favorites_db_path(self) -> Path:
        """Get the favorites database path.

        Returns:
            Path to the favorites.db file.
        """
        return self._data_dir / self.FAVORITES_DB

    def get_favorites_connection(self) -> sqlite3.Connection:
        """Get a connection to the favorites database.

        Returns:
            SQLite connection to favorites.db
        """
        conn = sqlite3.connect(str(self.favorites_db_path))
        conn.row_factory = sqlite3.Row
        self._init_favorites_table(conn)
        return conn

    def _init_favorites_table(self, conn: sqlite3.Connection) -> None:
        """Initialize the favorites table if it doesn't exist.

        Args:
            conn: SQLite connection
        """
        cursor = conn.cursor()
        cursor.execute("""
            CREATE TABLE IF NOT EXISTS favorites (
                session_id TEXT PRIMARY KEY,
                source TEXT NOT NULL,
                added_at INTEGER NOT NULL
            )
        """)
        conn.commit()
