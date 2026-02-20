package session

import "testing"

func TestNormalizeTitle_SingleLineAndWhitespace(t *testing.T) {
	raw := "  hello\tworld \n from\rclaude  "
	got := NormalizeTitle(raw, NormalizedTitleWidth)
	want := "hello world from claude"
	if got != want {
		t.Fatalf("expected '%s', got '%s'", want, got)
	}
}

func TestNormalizeTitle_EmptyFallback(t *testing.T) {
	got := NormalizeTitle("   \n\t  ", NormalizedTitleWidth)
	if got != "(untitled session)" {
		t.Fatalf("expected fallback title, got '%s'", got)
	}
}

func TestNormalizeTitle_TruncateByWidth(t *testing.T) {
	raw := "这是一个很长很长很长很长很长很长很长很长很长很长很长很长很长很长很长很长很长很长很长很长的标题"
	got := NormalizeTitle(raw, 20)
	if len([]rune(got)) == 0 {
		t.Fatal("expected non-empty title")
	}
	if got[len(got)-len("…"):] != "…" {
		t.Fatalf("expected ellipsis suffix, got '%s'", got)
	}
}

func TestNormalizeSession_Idempotent(t *testing.T) {
	origin := Session{
		ID:          "",
		Title:       " \n\thello ",
		SourceTool:  SourceClaude,
		ProjectPath: " /home/lee/project ",
		LastUpdated: -1,
	}

	once := NormalizeSession(origin)
	twice := NormalizeSession(once)

	if once != twice {
		t.Fatalf("NormalizeSession should be idempotent: once=%+v twice=%+v", once, twice)
	}
}

func TestCanonicalizeProjectPath_DecodeDashedHomePath(t *testing.T) {
	got := CanonicalizeProjectPath("-home-lee-11MyProjrct-projectA")
	want := "/home/lee/11MyProjrct/projectA"
	if got != want {
		t.Fatalf("expected '%s', got '%s'", want, got)
	}
}

func TestCanonicalizeProjectPath_TrimAndClean(t *testing.T) {
	got := CanonicalizeProjectPath(" /home/lee/proj/../projA/ ")
	want := "/home/lee/projA"
	if got != want {
		t.Fatalf("expected '%s', got '%s'", want, got)
	}
}
