package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Core Data timestamp の基準日（2001年1月1日）
var coreDataEpoch = time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

// HistoryVisit は個別の訪問記録を表す
type HistoryVisit struct {
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Domain    string    `json:"domain"`
	VisitTime time.Time `json:"visit_time"`
}

// DomainStats はドメイン別の統計情報
type DomainStats struct {
	Domain     string `json:"domain"`
	VisitCount int    `json:"visit_count"`
}

// HourlyStats は時間帯別の統計情報
type HourlyStats struct {
	Hour       int `json:"hour"`
	VisitCount int `json:"visit_count"`
}

// DailyStats は日別の統計情報
type DailyStats struct {
	Date       string `json:"date"`
	VisitCount int    `json:"visit_count"`
}

// AnalysisResult は分析結果全体を表す
type AnalysisResult struct {
	TotalVisits  int            `json:"total_visits"`
	RecentVisits []HistoryVisit `json:"recent_visits,omitempty"`
	DomainStats  []DomainStats  `json:"domain_stats,omitempty"`
	HourlyStats  []HourlyStats  `json:"hourly_stats,omitempty"`
	DailyStats   []DailyStats   `json:"daily_stats,omitempty"`
}

// visit_time を通常の時刻に変換
func convertCoreDataTimestamp(timestamp float64) time.Time {
	return coreDataEpoch.Add(time.Duration(timestamp * float64(time.Second)))
}

// getDBPath はSafari履歴DBのパスを取得
func getDBPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("ホームディレクトリの取得に失敗: %w", err)
	}
	return filepath.Join(homeDir, "Library", "Safari", "History.db"), nil
}

// openDB はSafari履歴DBを開く（読み取り専用）
func openDB(dbPath string) (*sql.DB, error) {
	// 読み取り専用モードで開く
	db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("データベースを開けませんでした: %w", err)
	}
	return db, nil
}

// getRecentVisits は最近の訪問履歴を取得
func getRecentVisits(db *sql.DB, limit int) ([]HistoryVisit, error) {
	query := `
		SELECT
			hi.url,
			COALESCE(hv.title, '') as title,
			COALESCE(hi.domain_expansion, '') as domain,
			hv.visit_time
		FROM history_visits hv
		JOIN history_items hi ON hv.history_item = hi.id
		ORDER BY hv.visit_time DESC
		LIMIT ?
	`
	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("履歴の取得に失敗: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var visits []HistoryVisit
	for rows.Next() {
		var v HistoryVisit
		var visitTime float64
		if err := rows.Scan(&v.URL, &v.Title, &v.Domain, &visitTime); err != nil {
			return nil, fmt.Errorf("行の読み取りに失敗: %w", err)
		}
		v.VisitTime = convertCoreDataTimestamp(visitTime)
		visits = append(visits, v)
	}
	return visits, nil
}

// getDomainStats はドメイン別の訪問統計を取得
func getDomainStats(db *sql.DB, limit int) ([]DomainStats, error) {
	query := `
		SELECT
			COALESCE(domain_expansion, 'その他') as domain,
			SUM(visit_count) as total_visits
		FROM history_items
		GROUP BY domain_expansion
		ORDER BY total_visits DESC
		LIMIT ?
	`
	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("ドメイン統計の取得に失敗: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var stats []DomainStats
	for rows.Next() {
		var s DomainStats
		if err := rows.Scan(&s.Domain, &s.VisitCount); err != nil {
			return nil, fmt.Errorf("行の読み取りに失敗: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// getHourlyStats は時間帯別の訪問統計を取得
func getHourlyStats(db *sql.DB) ([]HourlyStats, error) {
	query := `
		SELECT visit_time FROM history_visits
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("時間帯統計の取得に失敗: %w", err)
	}
	defer func() { _ = rows.Close() }()

	hourCounts := make(map[int]int)
	for rows.Next() {
		var visitTime float64
		if err := rows.Scan(&visitTime); err != nil {
			return nil, fmt.Errorf("行の読み取りに失敗: %w", err)
		}
		t := convertCoreDataTimestamp(visitTime)
		hourCounts[t.Hour()]++
	}

	var stats []HourlyStats
	for hour := 0; hour < 24; hour++ {
		stats = append(stats, HourlyStats{
			Hour:       hour,
			VisitCount: hourCounts[hour],
		})
	}
	return stats, nil
}

// getDailyStats は日別の訪問統計を取得（過去N日間）
func getDailyStats(db *sql.DB, days int) ([]DailyStats, error) {
	query := `
		SELECT visit_time FROM history_visits
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("日別統計の取得に失敗: %w", err)
	}
	defer func() { _ = rows.Close() }()

	dateCounts := make(map[string]int)
	cutoff := time.Now().AddDate(0, 0, -days)

	for rows.Next() {
		var visitTime float64
		if err := rows.Scan(&visitTime); err != nil {
			return nil, fmt.Errorf("行の読み取りに失敗: %w", err)
		}
		t := convertCoreDataTimestamp(visitTime)
		if t.After(cutoff) {
			dateStr := t.Format("2006-01-02")
			dateCounts[dateStr]++
		}
	}

	var stats []DailyStats
	for date, count := range dateCounts {
		stats = append(stats, DailyStats{
			Date:       date,
			VisitCount: count,
		})
	}

	// 日付でソート
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Date > stats[j].Date
	})

	return stats, nil
}

// getTotalVisits は総訪問数を取得
func getTotalVisits(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM history_visits").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("総訪問数の取得に失敗: %w", err)
	}
	return count, nil
}

// printTextOutput はテキスト形式で結果を出力
func printTextOutput(result AnalysisResult, showHistory, showDomains, showHourly, showDaily bool) {
	fmt.Printf("\n📊 Safari 履歴分析結果\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("総訪問数: %d\n\n", result.TotalVisits)

	if showHistory && len(result.RecentVisits) > 0 {
		fmt.Printf("📝 最近の訪問履歴\n")
		fmt.Printf("─────────────────────────────────────────\n")
		for _, v := range result.RecentVisits {
			title := v.Title
			if title == "" {
				title = "(タイトルなし)"
			}
			if len(title) > 50 {
				title = title[:47] + "..."
			}
			fmt.Printf("  %s  %s\n", v.VisitTime.Format("2006-01-02 15:04"), title)
			if v.Domain != "" {
				fmt.Printf("              📍 %s\n", v.Domain)
			}
		}
		fmt.Println()
	}

	if showDomains && len(result.DomainStats) > 0 {
		fmt.Printf("🌐 ドメイン別訪問数 (Top %d)\n", len(result.DomainStats))
		fmt.Printf("─────────────────────────────────────────\n")
		maxCount := result.DomainStats[0].VisitCount
		for _, s := range result.DomainStats {
			barLen := int(float64(s.VisitCount) / float64(maxCount) * 20)
			bar := strings.Repeat("█", barLen)
			fmt.Printf("  %-20s %s %d\n", s.Domain, bar, s.VisitCount)
		}
		fmt.Println()
	}

	if showHourly && len(result.HourlyStats) > 0 {
		fmt.Printf("⏰ 時間帯別訪問数\n")
		fmt.Printf("─────────────────────────────────────────\n")
		maxCount := 0
		for _, s := range result.HourlyStats {
			if s.VisitCount > maxCount {
				maxCount = s.VisitCount
			}
		}
		for _, s := range result.HourlyStats {
			barLen := 0
			if maxCount > 0 {
				barLen = int(float64(s.VisitCount) / float64(maxCount) * 20)
			}
			bar := strings.Repeat("█", barLen)
			fmt.Printf("  %02d:00  %s %d\n", s.Hour, bar, s.VisitCount)
		}
		fmt.Println()
	}

	if showDaily && len(result.DailyStats) > 0 {
		fmt.Printf("📅 日別訪問数 (過去%d日間)\n", len(result.DailyStats))
		fmt.Printf("─────────────────────────────────────────\n")
		maxCount := 0
		for _, s := range result.DailyStats {
			if s.VisitCount > maxCount {
				maxCount = s.VisitCount
			}
		}
		for _, s := range result.DailyStats {
			barLen := 0
			if maxCount > 0 {
				barLen = int(float64(s.VisitCount) / float64(maxCount) * 20)
			}
			bar := strings.Repeat("█", barLen)
			fmt.Printf("  %s  %s %d\n", s.Date, bar, s.VisitCount)
		}
		fmt.Println()
	}
}

func main() {
	// コマンドラインフラグの定義
	jsonOutput := flag.Bool("json", false, "JSON形式で出力")
	limit := flag.Int("limit", 20, "表示する履歴の件数")
	domainLimit := flag.Int("domains", 10, "表示するドメイン統計の件数")
	days := flag.Int("days", 7, "日別統計の対象日数")

	showHistory := flag.Bool("history", false, "履歴一覧を表示")
	showDomains := flag.Bool("domain-stats", false, "ドメイン別統計を表示")
	showHourly := flag.Bool("hourly", false, "時間帯別統計を表示")
	showDaily := flag.Bool("daily", false, "日別統計を表示")
	showAll := flag.Bool("all", false, "全ての分析結果を表示")

	flag.Parse()

	// -all が指定された場合は全て表示
	if *showAll {
		*showHistory = true
		*showDomains = true
		*showHourly = true
		*showDaily = true
	}

	// 何も指定されていない場合はデフォルトで履歴を表示
	if !*showHistory && !*showDomains && !*showHourly && !*showDaily {
		*showHistory = true
	}

	// Safari履歴DBのパスを取得
	dbPath, err := getDBPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		os.Exit(1)
	}

	// DBを開く
	db, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	// 分析結果を格納
	var result AnalysisResult

	// 総訪問数を取得
	result.TotalVisits, err = getTotalVisits(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		os.Exit(1)
	}

	// 各種統計を取得
	if *showHistory {
		result.RecentVisits, err = getRecentVisits(db, *limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
			os.Exit(1)
		}
	}

	if *showDomains {
		result.DomainStats, err = getDomainStats(db, *domainLimit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
			os.Exit(1)
		}
	}

	if *showHourly {
		result.HourlyStats, err = getHourlyStats(db)
		if err != nil {
			fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
			os.Exit(1)
		}
	}

	if *showDaily {
		result.DailyStats, err = getDailyStats(db, *days)
		if err != nil {
			fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
			os.Exit(1)
		}
	}

	// 出力
	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "JSON出力エラー: %v\n", err)
			os.Exit(1)
		}
	} else {
		printTextOutput(result, *showHistory, *showDomains, *showHourly, *showDaily)
	}
}
