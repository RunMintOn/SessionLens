import os
import subprocess
import shutil
from pathlib import Path
from typing import List, Dict, Any, Optional

from textual.app import App
from textual.widgets import Tree, Button, Header, Footer, Static, Label
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.events import Click, Resize
from rich.text import Text

# Import our modules
from scanner import scan_all
from config import Config
from models import Session

COLORS = {
    "opencode": "#86EFAC",
    "qwen": "#93C5FD",
    "claude": "#FDBA74",
}


def truncate_title(title: str, max_length: int = 30) -> str:
    """Truncate long titles with ellipsis"""
    if len(title) <= max_length:
        return title
    return title[:max_length-3] + "..."


def format_label(title: str, source: str, is_favorite: bool, terminal_width: int) -> Text:
    min_width = 25
    if terminal_width < min_width:
        terminal_width = min_width
    
    fav_size = 2
    source_size = 8
    right_padding = 2
    scrollbar_width = 1
    
    available = terminal_width - fav_size - source_size - right_padding - scrollbar_width - 4
    
    if available < 8:
        available = 8
    
    if len(title) > available:
        title = title[:available-3] + "..."
    
    padded_title = title.ljust(available)
    fav_marker = "★ " if is_favorite else "· "
    color = COLORS.get(source.lower(), "#FFFFFF")
    label = Text(fav_marker + padded_title + " ")
    label.append(source.lower(), style=color)
    return label


def simplify_path(path: str) -> str:
    """Simplify project path (e.g., /home/user/project -> ~/project)"""
    if not path:
        return "未分类"
    
    home = str(Path.home())
    if path.startswith(home):
        return "~" + path[len(home):]
    
    parts = Path(path).parts
    if len(parts) > 2:
        return f"~/{parts[-2]}/{parts[-1]}"
    return path


class SessionManagerApp(App):
    """Main TUI application for session management"""
    
    CSS = """
    Screen {
        background: $surface;
    }
    .panel {
        border: solid $border;
        padding: 1;
    }
    .left-panel {
        width: 60%;
    }
    .right-panel {
        width: 40%;
    }
    .panel-title {
        text-style: bold;
        color: $accent;
    }
    .filter-buttons {
        height: auto;
        margin-bottom: 1;
    }
    .sessions-tree {
        height: 100%;
    }
    .favorites-list {
        height: 100%;
    }
    """
    
    BINDINGS = [
        Binding("q", "quit", "退出"),
        Binding("r", "refresh", "刷新"),
        Binding("c", "toggle_favorite", "收藏"),
        Binding("up", "cursor_up", "向上"),
        Binding("down", "cursor_down", "向下"),
        Binding("enter", "restore_session", "恢复"),
    ]
    
    def __init__(self):
        super().__init__()
        self.config = Config()
        self.sessions: List[Session] = []
        self.filtered_sessions: List[Session] = []
        self.current_filter = "all"
        self.favorites: List[Session] = []
        self.favorite_ids: Dict[str, str] = {}  # session_id -> source
        self.projects: Dict[str, Dict] = {}
        self.selected_session: Optional[Session] = None
        self._resize_timer = None  # Debounce timer for resize
    
    def compose(self):
        """Compose the UI layout"""
        yield Header(show_clock=True)
        
        # Main content area with horizontal layout
        with Horizontal():
            # Left panel - Sessions
            with Vertical(classes="panel left-panel"):
                yield Label("SESSIONS", classes="panel-title")
                
                # Filter buttons
                with Horizontal(classes="filter-buttons"):
                    yield Button("全部", id="filter-all", variant="primary")
                    yield Button("Claude", id="filter-claude")
                    yield Button("OpenCode", id="filter-opencode")
                    yield Button("Qwen", id="filter-qwen")
                
                yield Tree("", id="sessions-tree", classes="sessions-tree")
            
            # Right panel - Favorites
            with Vertical(classes="panel right-panel"):
                yield Label("FAVORITES", classes="panel-title")
                yield Static("No favorites yet", id="favorites-list", classes="favorites-list")
        
        yield Footer()
    
    def on_mount(self) -> None:
        """Called when the app is mounted"""
        self.load_favorites_from_db()
        self.refresh_sessions()
        self.update_favorites()
    
    def load_favorites_from_db(self) -> None:
        """Load favorites from the config database"""
        conn = self.config.get_favorites_connection()
        cursor = conn.cursor()
        cursor.execute("SELECT session_id, source FROM favorites")
        rows = cursor.fetchall()
        conn.close()
        
        # Build a set of favorite session IDs for quick lookup
        self.favorite_ids = {row['session_id']: row['source'] for row in rows}
    
    def refresh_sessions(self) -> None:
        """Refresh session data and update the tree"""
        self.sessions = scan_all()
        self.filtered_sessions = self.sessions
        
        # Group sessions by project
        self.projects: Dict[str, Dict] = {}
        for session in self.sessions:
            project_path = session.project_path
            if project_path not in self.projects:
                project_name = os.path.basename(project_path) if project_path else "未分类"
                self.projects[project_path] = {
                    'name': project_name,
                    'path': project_path,
                    'sessions': []
                }
            self.projects[project_path]['sessions'].append(session)
        
        # Update the tree
        tree = self.query_one("#sessions-tree")
        tree.clear()
        
        for project_data in self.projects.values():
            project_node = tree.root.add(project_data['name'])
            for session in project_data['sessions']:
                fav_marker = "★" if session.id in self.favorite_ids else "·"
                label = f"{fav_marker} {truncate_title(session.title)} [{session.source_tool}]"
                node = project_node.add(label)
                node.data = session  # Store session reference for selection
        
        # Expand all by default
        tree.root.expand_all()
        
        # Apply current filter
        self.apply_filter()
    
    def apply_filter(self) -> None:
        """Apply the current filter to sessions"""
        if self.current_filter == "all":
            self.filtered_sessions = self.sessions
        else:
            self.filtered_sessions = [s for s in self.sessions if s.source_tool.lower() == self.current_filter.lower()]
        
        # Refresh tree display
        self._refresh_tree_display()
    
    def _refresh_tree_display(self) -> None:
        """Refresh just the tree display without rescanning"""
        tree = self.query_one("#sessions-tree")
        tree.clear()
        
        terminal_width = tree.size.width
        
        if terminal_width < 30:
            terminal_width = 40
        
        # 禁用根节点展开（隐藏顶层空节点）
        tree.root.allow_expand = False
        
        for project_data in self.projects.values():
            # Filter sessions for this project
            project_sessions = [
                s for s in project_data['sessions']
                if self.current_filter == "all" or s.source_tool.lower() == self.current_filter.lower()
            ]
            if not project_sessions:
                continue
            
            # 直接添加到 tree.root，不加顶层节点
            project_node = tree.root.add(project_data['name'])
            project_node.allow_expand = True  # 项目节点可展开
            for session in project_sessions:
                label = format_label(session.title, session.source_tool, session.id in self.favorite_ids, terminal_width)
                node = project_node.add(label)
                node.allow_expand = False  # 禁用会话展开
                node.data = session
        
        tree.root.expand_all()
    
    def add_favorite_to_db(self, session_id: str, source: str) -> None:
        """Add a session to favorites in the database"""
        import time
        conn = self.config.get_favorites_connection()
        cursor = conn.cursor()
        cursor.execute(
            "INSERT OR REPLACE INTO favorites (session_id, source, added_at) VALUES (?, ?, ?)",
            (session_id, source, int(time.time()))
        )
        conn.commit()
        conn.close()
    
    def remove_favorite_from_db(self, session_id: str) -> None:
        """Remove a session from favorites in the database"""
        conn = self.config.get_favorites_connection()
        cursor = conn.cursor()
        cursor.execute("DELETE FROM favorites WHERE session_id = ?", (session_id,))
        conn.commit()
        conn.close()
    
    def on_button_pressed(self, event: Button.Pressed) -> None:
        """Handle filter button clicks"""
        button_id = event.button.id
        if button_id == "filter-all":
            self.current_filter = "all"
            self.apply_filter()
        elif button_id == "filter-claude":
            self.current_filter = "claude"
            self.apply_filter()
        elif button_id == "filter-opencode":
            self.current_filter = "opencode"
            self.apply_filter()
        elif button_id == "filter-qwen":
            self.current_filter = "qwen"
            self.apply_filter()
    
    def on_tree_node_selected(self, event: Tree.NodeSelected) -> None:
        """Handle tree node selection"""
        node = event.node
        if node and node.data:
            # Node data contains the session
            self.selected_session = node.data
    
    def update_favorites(self) -> None:
        """Update the favorites display"""
        favorites_list = self.query_one("#favorites-list")
        
        # Build favorites list from favorite_ids
        self.favorites = []
        for session in self.sessions:
            if session.id in self.favorite_ids:
                self.favorites.append(session)
        
        if not self.favorites:
            favorites_list.update("No favorites yet")
            return
        
        favorites_content = "\n".join([
            f"★ {truncate_title(fav.title)}\n  [{fav.source_tool}] · {simplify_path(fav.project_path)}"
            for fav in self.favorites
        ])
        favorites_list.update(favorites_content)
    
    def action_refresh(self) -> None:
        """Refresh button action"""
        self.refresh_sessions()
        self.update_favorites()
    
    def on_resize(self, event: Resize) -> None:
        if self._resize_timer:
            self._resize_timer.stop()
        self._resize_timer = self.set_timer(0.2, self._refresh_tree_display)
    
    def action_toggle_favorite(self) -> None:
        """Toggle favorite for selected session"""
        if not self.selected_session:
            # Try to get selected session from tree
            tree = self.query_one("#sessions-tree")
            if tree.cursor_node and tree.cursor_node.data:
                self.selected_session = tree.cursor_node.data
            else:
                return
        
        session = self.selected_session
        if not session:
            return
        
        session_id = session.id
        
        if session_id in self.favorite_ids:
            # Remove from favorites
            self.remove_favorite_from_db(session_id)
            del self.favorite_ids[session_id]
        else:
            # Add to favorites
            self.add_favorite_to_db(session_id, session.source_tool)
            self.favorite_ids[session_id] = session.source_tool
        
        # Refresh display
        self._refresh_tree_display()
        self.update_favorites()
    
    def action_restore_session(self) -> None:
        """Restore selected session"""
        if not self.selected_session:
            # Try to get selected session from tree
            tree = self.query_one("#sessions-tree")
            if tree.cursor_node and tree.cursor_node.data:
                self.selected_session = tree.cursor_node.data
            else:
                return
        
        session = self.selected_session
        if not session:
            return
        
        # Build restore command based on source tool
        tool = session.source_tool.lower()
        session_id = session.id
        
        if tool == "claude":
            cmd = ["claude", "-r", session_id]
        elif tool == "opencode":
            cmd = ["opencode", "-s", session_id]
        elif tool == "qwen":
            cmd = ["qwen", "-r", session_id]
        else:
            return
        
        # Check if tool is installed
        if not shutil.which(cmd[0]):
            self.notify(f"{cmd[0]} is not installed")
            return
        
        # Launch restore command (don't wait for it)
        subprocess.Popen(cmd, start_new_session=True)
        self.notify(f"Restoring session: {session.title}")


if __name__ == "__main__":
    app = SessionManagerApp()
    app.run()
