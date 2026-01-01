package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// TestExtractDomain はURLからドメイン抽出のテスト
func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"HTTPS URL", "https://www.example.com/path", "www.example.com"},
		{"HTTP URL", "http://example.com/", "example.com"},
		{"ポート付き", "https://localhost:8080/api", "localhost"},
		{"クエリ付き", "https://google.com?q=test", "google.com"},
		{"フラグメント付き", "https://site.com#section", "site.com"},
		{"パスなし", "https://domain.com", "domain.com"},
		{"サブドメイン", "https://sub.domain.example.com/page", "sub.domain.example.com"},
		{"プロトコルなし", "example.com/path", ""},
		{"空文字列", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDomain(tt.url)
			if got != tt.want {
				t.Errorf("extractDomain(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// TestConvertCoreDataTimestamp はタイムスタンプ変換のテスト
func TestConvertCoreDataTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		timestamp float64
		want      time.Time
	}{
		{
			name:      "基準日（2001-01-01 00:00:00 UTC）",
			timestamp: 0,
			want:      time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "1日後",
			timestamp: 86400,
			want:      time.Date(2001, 1, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "2025-01-01 00:00:00 UTC",
			timestamp: 757382400, // 24年分の秒数
			want:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "小数点以下（ミリ秒）",
			timestamp: 100.5,
			want:      coreDataEpoch.Add(time.Duration(100.5 * float64(time.Second))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertCoreDataTimestamp(tt.timestamp)
			if !got.Equal(tt.want) {
				t.Errorf("convertCoreDataTimestamp(%v) = %v, want %v", tt.timestamp, got, tt.want)
			}
		})
	}
}

// setupTestDB はテスト用のインメモリDBを作成
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("テストDB作成に失敗: %v", err)
	}

	// テーブル作成
	_, err = db.Exec(`
		CREATE TABLE history_items (
			id INTEGER PRIMARY KEY,
			url TEXT NOT NULL UNIQUE,
			domain_expansion TEXT,
			visit_count INTEGER DEFAULT 0
		);
		CREATE TABLE history_visits (
			id INTEGER PRIMARY KEY,
			history_item INTEGER,
			visit_time REAL,
			title TEXT,
			FOREIGN KEY (history_item) REFERENCES history_items(id)
		);
	`)
	if err != nil {
		t.Fatalf("テーブル作成に失敗: %v", err)
	}

	return db
}

// insertTestData はテストデータを挿入
func insertTestData(t *testing.T, db *sql.DB) {
	// history_items
	_, err := db.Exec(`
		INSERT INTO history_items (id, url, domain_expansion, visit_count) VALUES
		(1, 'https://github.com/test', 'github', 10),
		(2, 'https://youtube.com/watch', 'youtube', 25),
		(3, 'https://google.com/search', 'google', 15),
		(4, 'https://example.com', NULL, 5);
	`)
	if err != nil {
		t.Fatalf("history_items挿入に失敗: %v", err)
	}

	// history_visits（Core Dataタイムスタンプ使用）
	// 2025-01-01 10:00:00 UTC = 757418400秒
	baseTime := 757418400.0
	_, err = db.Exec(`
		INSERT INTO history_visits (id, history_item, visit_time, title) VALUES
		(1, 1, ?, 'GitHub - Test Repo'),
		(2, 2, ?, 'YouTube Video'),
		(3, 3, ?, 'Google Search'),
		(4, 1, ?, 'GitHub - Another Page'),
		(5, 2, ?, 'YouTube - Music');
	`, baseTime, baseTime+3600, baseTime+7200, baseTime+86400, baseTime+90000)
	if err != nil {
		t.Fatalf("history_visits挿入に失敗: %v", err)
	}
}

// TestGetTotalVisits は総訪問数取得のテスト
func TestGetTotalVisits(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	insertTestData(t, db)

	count, err := getTotalVisits(db)
	if err != nil {
		t.Fatalf("getTotalVisits失敗: %v", err)
	}

	if count != 5 {
		t.Errorf("getTotalVisits() = %d, want 5", count)
	}
}

// TestGetRecentVisits は最近の訪問履歴取得のテスト
func TestGetRecentVisits(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	insertTestData(t, db)

	visits, err := getRecentVisits(db, 3, SearchFilter{})
	if err != nil {
		t.Fatalf("getRecentVisits失敗: %v", err)
	}

	if len(visits) != 3 {
		t.Errorf("getRecentVisits(3) returned %d items, want 3", len(visits))
	}

	// 最新のものが最初に来ているか確認
	if visits[0].Title != "YouTube - Music" {
		t.Errorf("最新の訪問タイトルが期待と異なる: got %s", visits[0].Title)
	}
}

// TestGetRecentVisitsWithKeywordFilter はキーワード検索のテスト
func TestGetRecentVisitsWithKeywordFilter(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	insertTestData(t, db)

	filter := SearchFilter{Keyword: "GitHub"}
	visits, err := getRecentVisits(db, 10, filter)
	if err != nil {
		t.Fatalf("getRecentVisits失敗: %v", err)
	}

	if len(visits) != 2 {
		t.Errorf("キーワード'GitHub'で%d件、期待は2件", len(visits))
	}
}

// TestGetRecentVisitsWithDomainFilter はドメインフィルタのテスト
func TestGetRecentVisitsWithDomainFilter(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	insertTestData(t, db)

	filter := SearchFilter{Domain: "youtube"}
	visits, err := getRecentVisits(db, 10, filter)
	if err != nil {
		t.Fatalf("getRecentVisits失敗: %v", err)
	}

	if len(visits) != 2 {
		t.Errorf("ドメイン'youtube'で%d件、期待は2件", len(visits))
	}
}

// TestGetDomainStats はドメイン別統計取得のテスト
func TestGetDomainStats(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	insertTestData(t, db)

	stats, err := getDomainStats(db, 10, SearchFilter{})
	if err != nil {
		t.Fatalf("getDomainStats失敗: %v", err)
	}

	// URLから抽出したドメイン: github.com, youtube.com, google.com, example.com
	if len(stats) != 4 {
		t.Errorf("getDomainStats() returned %d items, want 4", len(stats))
	}

	// 訪問数順にソートされているか（youtube.com: 25）
	if stats[0].Domain != "youtube.com" || stats[0].VisitCount != 25 {
		t.Errorf("最多訪問ドメインが期待と異なる: got %s (%d)", stats[0].Domain, stats[0].VisitCount)
	}
}

// TestGetDomainStatsWithIgnoreList はイグノアリスト付きドメイン統計取得のテスト
func TestGetDomainStatsWithIgnoreList(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	insertTestData(t, db)

	// "youtube"を指定すると youtube.com がマッチする
	filter := SearchFilter{IgnoreDomains: []string{"youtube.com"}}
	stats, err := getDomainStats(db, 10, filter)
	if err != nil {
		t.Fatalf("getDomainStats失敗: %v", err)
	}

	// youtube.comが除外されているので3件（github.com, google.com, example.com）
	if len(stats) != 3 {
		t.Errorf("getDomainStats() with ignore list returned %d items, want 3", len(stats))
	}

	// youtube.comが含まれていないか確認
	for _, s := range stats {
		if s.Domain == "youtube.com" {
			t.Error("イグノアリストで指定したドメインが結果に含まれている")
		}
	}
}

// TestGetHourlyStats は時間帯別統計取得のテスト
func TestGetHourlyStats(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	insertTestData(t, db)

	stats, err := getHourlyStats(db, SearchFilter{})
	if err != nil {
		t.Fatalf("getHourlyStats失敗: %v", err)
	}

	// 24時間分あるか
	if len(stats) != 24 {
		t.Errorf("getHourlyStats() returned %d items, want 24", len(stats))
	}

	// 10時台に訪問があるか確認（テストデータの最初の訪問）
	if stats[10].VisitCount == 0 {
		t.Error("10時台の訪問数が0になっている")
	}
}

// TestGetHourlyStatsWithDomainFilter はドメインフィルタ付き時間帯統計のテスト
func TestGetHourlyStatsWithDomainFilter(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	insertTestData(t, db)

	filter := SearchFilter{Domain: "github"}
	stats, err := getHourlyStats(db, filter)
	if err != nil {
		t.Fatalf("getHourlyStats失敗: %v", err)
	}

	// 合計訪問数を確認（githubは2件）
	total := 0
	for _, s := range stats {
		total += s.VisitCount
	}
	if total != 2 {
		t.Errorf("githubドメインの合計訪問数 = %d, want 2", total)
	}
}

// TestGetDailyStats は日別統計取得のテスト
func TestGetDailyStats(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	insertTestData(t, db)

	// 過去30日間の統計
	stats, err := getDailyStats(db, 30, SearchFilter{})
	if err != nil {
		t.Fatalf("getDailyStats失敗: %v", err)
	}

	// テストデータは2025-01-01と2025-01-02に訪問がある
	// ただし現在時刻から30日以内でない場合は0件になる可能性がある
	t.Logf("getDailyStats returned %d days", len(stats))
}

// TestAnalysisResultJSON はJSON出力フォーマットのテスト
func TestAnalysisResultJSON(t *testing.T) {
	result := AnalysisResult{
		TotalVisits: 100,
		RecentVisits: []HistoryVisit{
			{
				URL:       "https://example.com",
				Title:     "Example",
				Domain:    "example",
				VisitTime: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
			},
		},
		DomainStats: []DomainStats{
			{Domain: "example", VisitCount: 50},
		},
		HourlyStats: []HourlyStats{
			{Hour: 10, VisitCount: 20},
		},
		DailyStats: []DailyStats{
			{Date: "2025-01-01", VisitCount: 30},
		},
	}

	// JSONエンコード
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("JSONエンコード失敗: %v", err)
	}

	// JSONデコードして検証
	var decoded AnalysisResult
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("JSONデコード失敗: %v", err)
	}

	if decoded.TotalVisits != 100 {
		t.Errorf("TotalVisits = %d, want 100", decoded.TotalVisits)
	}

	if len(decoded.RecentVisits) != 1 {
		t.Errorf("RecentVisits length = %d, want 1", len(decoded.RecentVisits))
	}

	if decoded.RecentVisits[0].URL != "https://example.com" {
		t.Errorf("RecentVisits[0].URL = %s, want https://example.com", decoded.RecentVisits[0].URL)
	}
}

// TestAnalysisResultJSONOmitEmpty はomitemptyの動作テスト
func TestAnalysisResultJSONOmitEmpty(t *testing.T) {
	result := AnalysisResult{
		TotalVisits: 50,
		// 他のフィールドは空
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("JSONエンコード失敗: %v", err)
	}

	jsonStr := string(jsonBytes)

	// omitemptyにより空のフィールドは含まれないはず
	if contains(jsonStr, "recent_visits") {
		t.Error("空のrecent_visitsがJSONに含まれている")
	}
	if contains(jsonStr, "domain_stats") {
		t.Error("空のdomain_statsがJSONに含まれている")
	}
}

// TestGetDBPath はDBパス取得のテスト
func TestGetDBPath(t *testing.T) {
	path, err := getDBPath()
	if err != nil {
		t.Fatalf("getDBPath失敗: %v", err)
	}

	homeDir, _ := os.UserHomeDir()
	expected := homeDir + "/Library/Safari/History.db"

	if path != expected {
		t.Errorf("getDBPath() = %s, want %s", path, expected)
	}
}

// contains は文字列に部分文字列が含まれるかチェック
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestExtractBaseDomain はベースドメイン抽出のテスト
func TestExtractBaseDomain(t *testing.T) {
	tests := []struct {
		name   string
		domain string
		want   string
	}{
		{"シンプルなドメイン", "example.com", "example.com"},
		{"www付き", "www.example.com", "example.com"},
		{"サブドメイン付き", "mail.google.com", "google.com"},
		{"複数サブドメイン", "api.v2.example.com", "example.com"},
		{"日本の特殊TLD", "www.example.co.jp", "example.co.jp"},
		{"イギリスの特殊TLD", "shop.example.co.uk", "example.co.uk"},
		{"オーストラリアの特殊TLD", "api.example.com.au", "example.com.au"},
		{"ブラジルの特殊TLD", "www.example.com.br", "example.com.br"},
		{"アカデミックドメイン", "lib.example.ac.jp", "example.ac.jp"},
		{"政府ドメイン", "portal.example.gov", "example.gov"},
		{"組織ドメイン", "www.example.org", "example.org"},
		{"or.jpドメイン", "www.example.or.jp", "example.or.jp"},
		{"ne.jpドメイン", "api.example.ne.jp", "example.ne.jp"},
		{"短いドメイン", "a.com", "a.com"},
		{"2文字ドメイン", "io.com", "io.com"},
		{"TLDのみ", "com", "com"},
		{"空文字列", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBaseDomain(tt.domain)
			if got != tt.want {
				t.Errorf("extractBaseDomain(%q) = %q, want %q", tt.domain, got, tt.want)
			}
		})
	}
}

// TestGetHierarchicalDomainStats は階層的ドメイン統計取得のテスト
func TestGetHierarchicalDomainStats(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	// サブドメインを含むテストデータを挿入
	_, err := db.Exec(`
		INSERT INTO history_items (id, url, domain_expansion, visit_count) VALUES
		(10, 'https://www.google.com/search', 'www.google', 10),
		(11, 'https://mail.google.com/inbox', 'mail.google', 8),
		(12, 'https://docs.google.com/doc', 'docs.google', 5),
		(13, 'https://github.com/repo', 'github', 15),
		(14, 'https://api.github.com/v1', 'api.github', 3);
	`)
	if err != nil {
		t.Fatalf("history_items挿入に失敗: %v", err)
	}

	baseTime := 757418400.0
	_, err = db.Exec(`
		INSERT INTO history_visits (id, history_item, visit_time, title) VALUES
		(10, 10, ?, 'Google Search'),
		(11, 10, ?, 'Google Search 2'),
		(12, 11, ?, 'Gmail'),
		(13, 12, ?, 'Google Docs'),
		(14, 13, ?, 'GitHub Repo'),
		(15, 13, ?, 'GitHub Repo 2'),
		(16, 13, ?, 'GitHub Repo 3'),
		(17, 14, ?, 'GitHub API');
	`, baseTime, baseTime+100, baseTime+200, baseTime+300, baseTime+400, baseTime+500, baseTime+600, baseTime+700)
	if err != nil {
		t.Fatalf("history_visits挿入に失敗: %v", err)
	}

	stats, err := getHierarchicalDomainStats(db, 10, SearchFilter{})
	if err != nil {
		t.Fatalf("getHierarchicalDomainStats失敗: %v", err)
	}

	// google.com と github.com の2つのベースドメインがある
	if len(stats) < 2 {
		t.Errorf("getHierarchicalDomainStats() returned %d items, want at least 2", len(stats))
	}

	// 最初のエントリはgoogle.com（visit_count合計: 10+8+5=23）
	// visit_countはhistory_items.visit_countを使用
	if len(stats) > 0 && stats[0].BaseDomain != "google.com" {
		t.Errorf("最多訪問ドメインが期待と異なる: got %s, want google.com", stats[0].BaseDomain)
	}

	// github.comはサブドメインを持つ（visit_count合計: 15+3=18）
	var githubStats *HierarchicalDomainStats
	for i := range stats {
		if stats[i].BaseDomain == "github.com" {
			githubStats = &stats[i]
			break
		}
	}

	if githubStats != nil {
		if !githubStats.HasSubdomains {
			t.Error("github.comにはサブドメインがあるはず")
		}
		if len(githubStats.Subdomains) != 2 {
			t.Errorf("github.comのサブドメイン数 = %d, want 2", len(githubStats.Subdomains))
		}
		// 合計カウント（history_items.visit_count: 15+3=18）
		if githubStats.TotalCount != 18 {
			t.Errorf("github.comの合計訪問数 = %d, want 18", githubStats.TotalCount)
		}
	} else {
		t.Error("github.comの統計が見つからない")
	}

	// google.comもサブドメインを持つ（visit_count合計: 10+8+5=23）
	var googleStats *HierarchicalDomainStats
	for i := range stats {
		if stats[i].BaseDomain == "google.com" {
			googleStats = &stats[i]
			break
		}
	}

	if googleStats != nil {
		if !googleStats.HasSubdomains {
			t.Error("google.comにはサブドメインがあるはず")
		}
		if len(googleStats.Subdomains) != 3 {
			t.Errorf("google.comのサブドメイン数 = %d, want 3", len(googleStats.Subdomains))
		}
		// 合計カウント（history_items.visit_count: 10+8+5=23）
		if googleStats.TotalCount != 23 {
			t.Errorf("google.comの合計訪問数 = %d, want 23", googleStats.TotalCount)
		}
	} else {
		t.Error("google.comの統計が見つからない")
	}
}

// TestGetHierarchicalDomainStatsWithIgnoreList はイグノアリスト付き階層ドメイン統計のテスト
func TestGetHierarchicalDomainStatsWithIgnoreList(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO history_items (id, url, domain_expansion, visit_count) VALUES
		(20, 'https://www.google.com/search', 'www.google', 10),
		(21, 'https://github.com/repo', 'github', 15);
	`)
	if err != nil {
		t.Fatalf("history_items挿入に失敗: %v", err)
	}

	baseTime := 757418400.0
	_, err = db.Exec(`
		INSERT INTO history_visits (id, history_item, visit_time, title) VALUES
		(20, 20, ?, 'Google Search'),
		(21, 21, ?, 'GitHub Repo');
	`, baseTime, baseTime+100)
	if err != nil {
		t.Fatalf("history_visits挿入に失敗: %v", err)
	}

	// googleをイグノア
	filter := SearchFilter{IgnoreDomains: []string{"google"}}
	stats, err := getHierarchicalDomainStats(db, 10, filter)
	if err != nil {
		t.Fatalf("getHierarchicalDomainStats失敗: %v", err)
	}

	// google.comが除外されているので1件のみ
	if len(stats) != 1 {
		t.Errorf("getHierarchicalDomainStats with ignore returned %d items, want 1", len(stats))
	}

	if len(stats) > 0 && stats[0].BaseDomain != "github.com" {
		t.Errorf("残るドメインが期待と異なる: got %s, want github.com", stats[0].BaseDomain)
	}
}
