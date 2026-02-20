package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"agent-session-manager/session"
)

func formatTimeAgo(unixTime int64) string {
	diff := time.Now().Unix() - unixTime
	days := diff / (24 * 60 * 60)
	if days == 0 {
		return "today"
	} else if days == 1 {
		return "yesterday"
	}
	return fmt.Sprintf("%d days ago", days)
}

func formatSize(bytes int64) string {
	kb := float64(bytes) / 1024
	if kb >= 1024 {
		return fmt.Sprintf("%.1f MB", kb/1024)
	}
	return fmt.Sprintf("%.1f KB", kb)
}

func main() {
	// 测试路径：/mnt/c/Users/l3e (Windows WSL)
	// 实际目录名是：-mnt-c-Users-l3e
	testPath := "/mnt/c/Users/l3e"
	// 正确的编码：/ 替换为 -，然后确保开头有一个 -
	encodedPath := strings.ReplaceAll(testPath, "/", "-")
	// 确保开头有且只有一个 -
	if !strings.HasPrefix(encodedPath, "-") {
		encodedPath = "-" + encodedPath
	}
	
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println("验证 Go Claude Scanner - Windows WSL 路径")
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Printf("目标路径：%s\n", testPath)
	fmt.Printf("编码目录：%s\n", encodedPath)
	fmt.Println()

	// 创建扫描器
	scanner := session.NewClaudeScanner()
	
	// 扫描指定路径
	sessions, err := scanner.Scan(encodedPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "扫描错误：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("找到 %d 条会话\n\n", len(sessions))

	// 按时间排序
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastUpdated > sessions[j].LastUpdated
	})

	// 显示结果
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("Go 扫描结果:")
	fmt.Println(strings.Repeat("=", 80))
	
	for i, s := range sessions {
		title := s.Title
		if len(title) > 80 {
			title = title[:80] + "..."
		}
		
		// 获取文件大小
		home, _ := os.UserHomeDir()
		jsonlPath := filepath.Join(home, ".claude", "projects", encodedPath, s.ID+".jsonl")
		var sizeStr string
		if info, err := os.Stat(jsonlPath); err == nil {
			sizeStr = formatSize(info.Size())
		} else {
			sizeStr = "N/A"
		}
		
		timeStr := formatTimeAgo(s.LastUpdated)
		
		fmt.Printf("\n%d. %s\n", i+1, title)
		fmt.Printf("   %s · %s · %s\n", timeStr, s.ID, sizeStr)
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("预期结果（来自截图）:")
	fmt.Println(strings.Repeat("=", 80))
	expected := []struct{
		title string
		days string
		size string
	}{
		{"/clear", "3 days ago", "1.6 KB"},
		{"你现在在哪", "5 days ago", "1.3 MB"},
		{"什么是内存顺序", "5 days ago", "123.5 KB"},
		{"你现在是在 wsl 环境", "5 days ago", "847.7 KB"},
		{"你好", "6 days ago", "1.5 KB"},
		{"res", "6 days ago", "13.6 KB"},
		{"你在 wsl 环境", "6 days ago", "5.0 KB"},
		{"测试", "6 days ago", "243.1 KB"},
	}
	for i, e := range expected {
		fmt.Printf("%d. %s - %s · HEAD · %s\n", i+1, e.title, e.days, e.size)
	}
}
