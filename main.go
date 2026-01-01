package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Core Data timestamp の基準日（2001年1月1日）
var coreDataEpoch = time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

// visit_time を通常の時刻に変換
func convertCoreDataTimestamp(timestamp float64) time.Time {
	return coreDataEpoch.Add(time.Duration(timestamp * float64(time.Second)))
}

func main() {
	// Safariの履歴DBパス
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("ホームディレクトリの取得に失敗:", err)
		return
	}
	dbPath := filepath.Join(homeDir, "Library", "Safari", "History.db")

	fmt.Println("Safari履歴を読み込み中:", dbPath)
}
