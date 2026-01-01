package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

// SearchFilter は検索・フィルタ条件を表す
type SearchFilter struct {
	Keyword       string
	Domain        string
	From          time.Time
	To            time.Time
	IgnoreDomains []string
}

// AnalysisResult は分析結果全体を表す
type AnalysisResult struct {
	TotalVisits  int            `json:"total_visits"`
	RecentVisits []HistoryVisit `json:"recent_visits,omitempty"`
	DomainStats  []DomainStats  `json:"domain_stats,omitempty"`
	HourlyStats  []HourlyStats  `json:"hourly_stats,omitempty"`
	DailyStats   []DailyStats   `json:"daily_stats,omitempty"`
}

// Config はアプリケーション設定を表す
type Config struct {
	// 表示件数
	Limit       int
	DomainLimit int
	Days        int

	// 表示オプション
	ShowHistory bool
	ShowDomains bool
	ShowHourly  bool
	ShowDaily   bool

	// フィルタ
	Filter SearchFilter

	// 出力形式
	JSONOutput bool
	CSVOutput  bool
	TSVOutput  bool
	OutputFile string

	// モード
	Interactive bool
	Serve       bool
	Port        int
}

// exitWithError はエラーメッセージを出力して終了する
func exitWithError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
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
	return filepath.Join(homeDir, SafariHistoryPath), nil
}

// openDB はSafari履歴DBを開く（読み取り専用）
func openDB(dbPath string) (*sql.DB, error) {
	// 読み取り専用モードで開く
	db, err := sql.Open(SQLiteDriver, dbPath+SQLiteReadOnlyMode)
	if err != nil {
		return nil, fmt.Errorf("データベースを開けませんでした: %w", err)
	}
	return db, nil
}

// convertToTimestamp は時刻をCore Data timestamp形式に変換
func convertToTimestamp(t time.Time) float64 {
	return t.Sub(coreDataEpoch).Seconds()
}

// extractDomain はURLからドメイン（ホスト名）を抽出する
// domain_expansionがNULLまたは空の場合のフォールバック用
func extractDomain(urlStr string) string {
	// プロトコル部分を探す
	start := strings.Index(urlStr, "://")
	if start == -1 {
		return ""
	}
	start += 3

	// ホスト部分の終わりを探す（パス、クエリ、ポートのいずれか）
	rest := urlStr[start:]
	end := len(rest)
	for i, c := range rest {
		if c == '/' || c == '?' || c == ':' || c == '#' {
			end = i
			break
		}
	}

	return rest[:end]
}

// 履歴取得用のベースクエリ
const historyBaseQuery = `
	SELECT
		hi.url,
		COALESCE(hv.title, '') as title,
		COALESCE(hi.domain_expansion, '') as domain,
		hv.visit_time
	FROM history_visits hv
	JOIN history_items hi ON hv.history_item = hi.id
	WHERE 1=1`

// getRecentVisits は最近の訪問履歴を取得
func getRecentVisits(db *sql.DB, limit int, filter SearchFilter) ([]HistoryVisit, error) {
	qb := NewQueryBuilder(historyBaseQuery).
		WithFilter(filter).
		OrderByDesc("hv.visit_time").
		Limit(limit)

	query, args := qb.Build()
	return executeHistoryQuery(db, query, args)
}

// executeHistoryQuery は履歴クエリを実行して結果を返す
func executeHistoryQuery(db *sql.DB, query string, args []interface{}) ([]HistoryVisit, error) {
	rows, err := db.Query(query, args...)
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
		// domain_expansionが空の場合、URLからドメインを抽出
		if v.Domain == "" {
			v.Domain = extractDomain(v.URL)
		}
		visits = append(visits, v)
	}
	return visits, nil
}

// getDomainStats はドメイン別の訪問統計を取得（URLからドメインを抽出）
func getDomainStats(db *sql.DB, limit int, filter SearchFilter) ([]DomainStats, error) {
	// 全てのURLとvisit_countを取得
	query := `SELECT hi.url, hi.visit_count FROM history_items hi`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("ドメイン統計の取得に失敗: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// URLからドメインを抽出して集計
	domainCounts := make(map[string]int)
	for rows.Next() {
		var url string
		var visitCount int
		if err := rows.Scan(&url, &visitCount); err != nil {
			return nil, fmt.Errorf("行の読み取りに失敗: %w", err)
		}
		domain := extractDomain(url)
		if domain == "" {
			domain = "不明"
		}

		// イグノアリストチェック
		if shouldIgnoreDomain(domain, filter.IgnoreDomains) {
			continue
		}

		domainCounts[domain] += visitCount
	}

	// スライスに変換してソート
	var stats []DomainStats
	for domain, count := range domainCounts {
		stats = append(stats, DomainStats{Domain: domain, VisitCount: count})
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].VisitCount > stats[j].VisitCount
	})

	// limitで制限
	if limit > 0 && len(stats) > limit {
		stats = stats[:limit]
	}

	return stats, nil
}

// shouldIgnoreDomain はドメインがイグノアリストに含まれるかチェック
func shouldIgnoreDomain(domain string, ignoreDomains []string) bool {
	for _, ignored := range ignoreDomains {
		if ignored == "" {
			continue
		}
		// 完全一致
		if domain == ignored {
			return true
		}
		// ドメインが ignored で始まる（例: youtube → youtube.com にマッチ）
		if len(domain) > len(ignored) && domain[:len(ignored)+1] == ignored+"." {
			return true
		}
		// サブドメイン（末尾が .ignored、例: google → accounts.google.com にマッチ）
		if len(domain) > len(ignored)+1 && domain[len(domain)-len(ignored)-1:] == "."+ignored {
			return true
		}
		// サブドメイン + TLD（例: google → accounts.google.com にマッチ）
		if strings.Contains(domain, "."+ignored+".") {
			return true
		}
	}
	return false
}

// 訪問時刻取得用のベースクエリ
const visitTimeBaseQuery = `
	SELECT hv.visit_time FROM history_visits hv
	JOIN history_items hi ON hv.history_item = hi.id
	WHERE 1=1`

// getHourlyStats は時間帯別の訪問統計を取得
func getHourlyStats(db *sql.DB, filter SearchFilter) ([]HourlyStats, error) {
	qb := NewQueryBuilder(visitTimeBaseQuery).WithFilter(filter)
	query, args := qb.Build()

	rows, err := db.Query(query, args...)
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
func getDailyStats(db *sql.DB, days int, filter SearchFilter) ([]DailyStats, error) {
	qb := NewQueryBuilder(visitTimeBaseQuery).WithFilter(filter)
	query, args := qb.Build()

	rows, err := db.Query(query, args...)
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
			dateStr := t.Format(TimeFormatDate)
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

// writeCSV はCSV/TSV形式で結果を出力
func writeCSV(w io.Writer, result AnalysisResult, showHistory, showDomains, showHourly, showDaily bool, delimiter rune) error {
	writer := csv.NewWriter(w)
	writer.Comma = delimiter
	defer writer.Flush()

	// 履歴一覧
	if showHistory && len(result.RecentVisits) > 0 {
		if err := writer.Write([]string{"visit_time", "title", "domain", "url"}); err != nil {
			return err
		}
		for _, v := range result.RecentVisits {
			record := []string{
				v.VisitTime.Format(TimeFormatFull),
				v.Title,
				v.Domain,
				v.URL,
			}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
	}

	// ドメイン統計
	if showDomains && len(result.DomainStats) > 0 {
		if showHistory && len(result.RecentVisits) > 0 {
			if err := writer.Write([]string{}); err != nil {
				return err
			}
		}
		if err := writer.Write([]string{"domain", "visit_count"}); err != nil {
			return err
		}
		for _, s := range result.DomainStats {
			record := []string{s.Domain, fmt.Sprintf("%d", s.VisitCount)}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
	}

	// 時間帯統計
	if showHourly && len(result.HourlyStats) > 0 {
		if (showHistory && len(result.RecentVisits) > 0) || (showDomains && len(result.DomainStats) > 0) {
			if err := writer.Write([]string{}); err != nil {
				return err
			}
		}
		if err := writer.Write([]string{"hour", "visit_count"}); err != nil {
			return err
		}
		for _, s := range result.HourlyStats {
			record := []string{fmt.Sprintf("%02d:00", s.Hour), fmt.Sprintf("%d", s.VisitCount)}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
	}

	// 日別統計
	if showDaily && len(result.DailyStats) > 0 {
		if (showHistory && len(result.RecentVisits) > 0) || (showDomains && len(result.DomainStats) > 0) || (showHourly && len(result.HourlyStats) > 0) {
			if err := writer.Write([]string{}); err != nil {
				return err
			}
		}
		if err := writer.Write([]string{"date", "visit_count"}); err != nil {
			return err
		}
		for _, s := range result.DailyStats {
			record := []string{s.Date, fmt.Sprintf("%d", s.VisitCount)}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
	}

	return nil
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
			if len(title) > TitleTruncateLength {
				title = title[:TitleTruncateLength-3] + "..."
			}
			fmt.Printf("  %s  %s\n", v.VisitTime.Format(TimeFormatDateTime), title)
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
			barLen := int(float64(s.VisitCount) / float64(maxCount) * BarChartWidth)
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
				barLen = int(float64(s.VisitCount) / float64(maxCount) * BarChartWidth)
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
				barLen = int(float64(s.VisitCount) / float64(maxCount) * BarChartWidth)
			}
			bar := strings.Repeat("█", barLen)
			fmt.Printf("  %s  %s %d\n", s.Date, bar, s.VisitCount)
		}
		fmt.Println()
	}
}

// parseFlags はコマンドラインフラグを解析してConfigを返す
func parseFlags() Config {
	// コマンドラインフラグの定義
	jsonOutput := flag.Bool("json", false, "JSON形式で出力")
	limit := flag.Int("limit", DefaultHistoryLimit, "表示する履歴の件数")
	domainLimit := flag.Int("domains", DefaultDomainLimit, "表示するドメイン統計の件数")
	days := flag.Int("days", DefaultDailyDays, "日別統計の対象日数")

	showHistory := flag.Bool("history", false, "履歴一覧を表示")
	showDomains := flag.Bool("domain-stats", false, "ドメイン別統計を表示")
	showHourly := flag.Bool("hourly", false, "時間帯別統計を表示")
	showDaily := flag.Bool("daily", false, "日別統計を表示")
	showAll := flag.Bool("all", false, "全ての分析結果を表示")

	// 検索・フィルタオプション
	search := flag.String("search", "", "キーワード検索（URL・タイトル）")
	domain := flag.String("domain", "", "ドメインでフィルタ")
	fromDate := flag.String("from", "", "開始日（YYYY-MM-DD）")
	toDate := flag.String("to", "", "終了日（YYYY-MM-DD）")

	// エクスポートオプション
	csvOutput := flag.Bool("csv", false, "CSV形式で出力")
	tsvOutput := flag.Bool("tsv", false, "TSV形式で出力")
	outputFile := flag.String("output", "", "出力ファイルパス")

	// インタラクティブモード
	interactive := flag.Bool("interactive", false, "インタラクティブモードで起動")
	flag.BoolVar(interactive, "i", false, "インタラクティブモードで起動（-interactiveの短縮形）")

	// Webサーバーモード
	serve := flag.Bool("serve", false, "Webサーバーモードで起動")
	port := flag.Int("port", DefaultWebPort, "Webサーバーのポート番号")

	// イグノアリスト管理
	ignoreAdd := flag.String("ignore-add", "", "ドメインをイグノアリストに追加")
	ignoreRemove := flag.String("ignore-remove", "", "ドメインをイグノアリストから削除")
	ignoreList := flag.Bool("ignore-list", false, "イグノアリストを表示")
	noIgnore := flag.Bool("no-ignore", false, "イグノアリストを無視して実行")

	flag.Parse()

	// イグノアリスト管理コマンドの処理
	if *ignoreList {
		if err := PrintIgnoreList(); err != nil {
			exitWithError("エラー: %v\n", err)
		}
		os.Exit(0)
	}
	if *ignoreAdd != "" {
		if err := AddToIgnoreList(*ignoreAdd); err != nil {
			exitWithError("エラー: %v\n", err)
		}
		fmt.Printf("イグノアリストに追加しました: %s\n", *ignoreAdd)
		os.Exit(0)
	}
	if *ignoreRemove != "" {
		if err := RemoveFromIgnoreList(*ignoreRemove); err != nil {
			exitWithError("エラー: %v\n", err)
		}
		fmt.Printf("イグノアリストから削除しました: %s\n", *ignoreRemove)
		os.Exit(0)
	}

	// フィルタ条件を構築
	var filter SearchFilter
	filter.Keyword = *search
	filter.Domain = *domain

	if *fromDate != "" {
		t, err := time.Parse(TimeFormatDate, *fromDate)
		if err != nil {
			exitWithError("エラー: 開始日の形式が不正です（YYYY-MM-DD）: %v\n", err)
		}
		filter.From = t
	}
	if *toDate != "" {
		t, err := time.Parse(TimeFormatDate, *toDate)
		if err != nil {
			exitWithError("エラー: 終了日の形式が不正です（YYYY-MM-DD）: %v\n", err)
		}
		filter.To = t
	}

	// イグノアリストを読み込み
	if !*noIgnore {
		ignoreDomains, err := LoadIgnoreList()
		if err != nil {
			exitWithError("エラー: イグノアリストの読み込みに失敗: %v\n", err)
		}
		filter.IgnoreDomains = ignoreDomains
	}

	// 表示オプションの正規化
	history := *showHistory
	domains := *showDomains
	hourly := *showHourly
	daily := *showDaily

	// -all が指定された場合は全て表示
	if *showAll {
		history = true
		domains = true
		hourly = true
		daily = true
	}

	// 何も指定されていない場合はデフォルトで履歴を表示
	if !history && !domains && !hourly && !daily {
		history = true
	}

	return Config{
		Limit:       *limit,
		DomainLimit: *domainLimit,
		Days:        *days,
		ShowHistory: history,
		ShowDomains: domains,
		ShowHourly:  hourly,
		ShowDaily:   daily,
		Filter:      filter,
		JSONOutput:  *jsonOutput,
		CSVOutput:   *csvOutput,
		TSVOutput:   *tsvOutput,
		OutputFile:  *outputFile,
		Interactive: *interactive,
		Serve:       *serve,
		Port:        *port,
	}
}

// setupDatabase はデータベース接続を確立する
func setupDatabase() (*sql.DB, error) {
	dbPath, err := getDBPath()
	if err != nil {
		return nil, err
	}
	return openDB(dbPath)
}

// runInteractiveOrWebMode はインタラクティブまたはWebモードを実行する
func runInteractiveOrWebMode(db *sql.DB, config Config) error {
	if config.Interactive {
		return runInteractiveMode(db)
	}
	if config.Serve {
		server, err := NewWebServer(db, config.Port)
		if err != nil {
			return err
		}
		return server.Start()
	}
	return nil
}

// runCLIMode はCLIモードで分析を実行する
func runCLIMode(db *sql.DB, config Config) error {
	var result AnalysisResult
	var err error

	// 総訪問数を取得
	result.TotalVisits, err = getTotalVisits(db)
	if err != nil {
		return fmt.Errorf("総訪問数の取得に失敗: %w", err)
	}

	// 各種統計を取得
	if config.ShowHistory {
		result.RecentVisits, err = getRecentVisits(db, config.Limit, config.Filter)
		if err != nil {
			return fmt.Errorf("履歴の取得に失敗: %w", err)
		}
	}

	if config.ShowDomains {
		result.DomainStats, err = getDomainStats(db, config.DomainLimit, config.Filter)
		if err != nil {
			return fmt.Errorf("ドメイン統計の取得に失敗: %w", err)
		}
	}

	if config.ShowHourly {
		result.HourlyStats, err = getHourlyStats(db, config.Filter)
		if err != nil {
			return fmt.Errorf("時間帯統計の取得に失敗: %w", err)
		}
	}

	if config.ShowDaily {
		result.DailyStats, err = getDailyStats(db, config.Days, config.Filter)
		if err != nil {
			return fmt.Errorf("日別統計の取得に失敗: %w", err)
		}
	}

	// 出力処理
	return outputResult(result, config)
}

// outputResult は結果を指定された形式で出力する
func outputResult(result AnalysisResult, config Config) error {
	// 出力先を決定
	var output io.Writer = os.Stdout
	if config.OutputFile != "" {
		f, err := os.Create(config.OutputFile)
		if err != nil {
			return fmt.Errorf("ファイル作成エラー: %w", err)
		}
		defer func() { _ = f.Close() }()
		output = f
	}

	// 出力形式に応じて出力
	switch {
	case config.JSONOutput:
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("JSON出力エラー: %w", err)
		}
	case config.CSVOutput:
		if err := writeCSV(output, result, config.ShowHistory, config.ShowDomains, config.ShowHourly, config.ShowDaily, ','); err != nil {
			return fmt.Errorf("CSV出力エラー: %w", err)
		}
	case config.TSVOutput:
		if err := writeCSV(output, result, config.ShowHistory, config.ShowDomains, config.ShowHourly, config.ShowDaily, '\t'); err != nil {
			return fmt.Errorf("TSV出力エラー: %w", err)
		}
	default:
		printTextOutput(result, config.ShowHistory, config.ShowDomains, config.ShowHourly, config.ShowDaily)
	}

	return nil
}

func main() {
	config := parseFlags()

	db, err := setupDatabase()
	if err != nil {
		exitWithError("エラー: %v\n", err)
	}
	defer func() { _ = db.Close() }()

	// インタラクティブまたはWebモード
	if config.Interactive || config.Serve {
		if err := runInteractiveOrWebMode(db, config); err != nil {
			exitWithError("エラー: %v\n", err)
		}
		return
	}

	// CLIモード
	if err := runCLIMode(db, config); err != nil {
		exitWithError("エラー: %v\n", err)
	}
}
