package session

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
)

// NormalizedTitleWidth is the default max display width for normalized titles.
const NormalizedTitleWidth = 500

// NormalizeSession applies source-agnostic normalization for stable TUI rendering.
func NormalizeSession(in Session) Session {
	out := in
	out.Title = NormalizeTitle(out.Title, NormalizedTitleWidth)
	out.ProjectPath = NormalizeProjectPath(out.ProjectPath)
	if out.LastUpdated < 0 {
		out.LastUpdated = 0
	}
	if strings.TrimSpace(out.ID) == "" {
		out.ID = fmt.Sprintf("unknown-%s-%d", out.SourceTool, out.LastUpdated)
	}
	return out
}

// NormalizeTitle converts raw title text into a compact, single-line title.
func NormalizeTitle(raw string, maxWidth int) string {
	normalized := normalizeSingleLine(raw)
	if normalized == "" {
		normalized = "(untitled session)"
	}
	if maxWidth <= 0 {
		return normalized
	}
	return runewidth.Truncate(normalized, maxWidth, "…")
}

// NormalizeProjectPath sanitizes project path for grouping and display.
func NormalizeProjectPath(raw string) string {
	return CanonicalizeProjectPath(raw)
}

// CanonicalizeProjectPath normalizes project paths from different sources
// so grouping can merge sessions for the same real project directory.
func CanonicalizeProjectPath(raw string) string {
	trimmed := strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, raw))
	if trimmed == "" {
		return "(no project)"
	}

	decoded, ok := decodeDashedProjectPath(trimmed)
	if ok {
		trimmed = decoded
	}

	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == "" {
		return "(no project)"
	}
	return strings.TrimSuffix(cleaned, "/")
}

// decodeDashedProjectPath decodes known "dash-encoded" project directory names
// such as "-home-lee-11MyProjrct-foo" into "/home/lee/11MyProjrct/foo".
func decodeDashedProjectPath(raw string) (string, bool) {
	if strings.Contains(raw, "/") || strings.Contains(raw, "\\") {
		return "", false
	}
	if !strings.HasPrefix(raw, "-home-") && !strings.HasPrefix(raw, "-mnt-") {
		return "", false
	}

	parts := strings.Split(strings.TrimPrefix(raw, "-"), "-")
	if len(parts) < 2 {
		return "", false
	}
	return "/" + strings.Join(parts, "/"), true
}

func normalizeSingleLine(raw string) string {
	withoutControls := strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, raw)

	return strings.Join(strings.Fields(withoutControls), " ")
}
