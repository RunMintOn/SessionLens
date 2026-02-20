"""Data models for agent session manager."""

from dataclasses import dataclass, field
from datetime import datetime
from typing import Optional


@dataclass
class Session:
    """Represents an agent session."""
    id: str
    title: str
    source_tool: str
    project_path: str
    last_updated: int  # Unix timestamp

    def to_dict(self) -> dict:
        """Convert to dictionary for serialization."""
        return {
            'id': self.id,
            'title': self.title,
            'source_tool': self.source_tool,
            'project_path': self.project_path,
            'last_updated': self.last_updated,
        }


@dataclass
class Project:
    """Represents a project with sessions."""
    path: str
    sessions: list[Session] = field(default_factory=list)

    def add_session(self, session: Session) -> None:
        """Add a session to this project."""
        self.sessions.append(session)

    def to_dict(self) -> dict:
        """Convert to dictionary for serialization."""
        return {
            'path': self.path,
            'sessions': [s.to_dict() for s in self.sessions],
        }


@dataclass
class Favorite:
    """Represents a favorited session."""
    session_id: str
    source: str
    added_at: int  # Unix timestamp

    def to_dict(self) -> dict:
        """Convert to dictionary for serialization."""
        return {
            'session_id': self.session_id,
            'source': self.source,
            'added_at': self.added_at,
        }