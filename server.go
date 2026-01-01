package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"
)

//go:embed web/templates/*.html
var templatesFS embed.FS

//go:embed web/static/*
var staticFS embed.FS

// テンプレート関数
var templateFuncs = template.FuncMap{
	"formatTime": func(t time.Time) string {
		return t.Format(TimeFormatFull)
	},
	"formatDate": func(t time.Time) string {
		return t.Format(TimeFormatDate)
	},
	"truncate": func(s string, length int) string {
		if len(s) <= length {
			return s
		}
		return s[:length-3] + "..."
	},
	"percentage": func(count, max int) float64 {
		if max == 0 {
			return 0
		}
		return float64(count) / float64(max) * 100
	},
	"add": func(a, b int) int {
		return a + b
	},
	"sub": func(a, b int) int {
		return a - b
	},
	"seq": func(start, end int) []int {
		var result []int
		for i := start; i <= end; i++ {
			result = append(result, i)
		}
		return result
	},
}

// WebServer はWebサーバーの構造体
type WebServer struct {
	db            *sql.DB
	templates     *template.Template
	port          int
	ignoreDomains []string
}

// NewWebServer は新しいWebServerを作成
func NewWebServer(db *sql.DB, port int) (*WebServer, error) {
	tmpl, err := template.New("").Funcs(templateFuncs).ParseFS(templatesFS, "web/templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("テンプレートの解析に失敗: %w", err)
	}

	// イグノアリストを読み込み
	ignoreDomains, err := LoadIgnoreList()
	if err != nil {
		return nil, fmt.Errorf("イグノアリストの読み込みに失敗: %w", err)
	}

	return &WebServer{
		db:            db,
		templates:     tmpl,
		port:          port,
		ignoreDomains: ignoreDomains,
	}, nil
}

// Start はWebサーバーを起動
func (s *WebServer) Start() error {
	mux := http.NewServeMux()

	// ページハンドラー
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/history", s.handleHistory)
	mux.HandleFunc("/stats", s.handleStatsPage)

	// APIハンドラー
	mux.HandleFunc("/api/stats", s.handleAPIStats)
	mux.HandleFunc("/api/stats/hourly", s.handleAPIStatsHourly)
	mux.HandleFunc("/api/stats/daily", s.handleAPIStatsDaily)
	mux.HandleFunc("/api/history", s.handleAPIHistory)
	mux.HandleFunc("/api/domains", s.handleAPIDomains)

	// 静的ファイル
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Web server starting at http://localhost%s", addr)
	return http.ListenAndServe(addr, mux)
}

// DashboardData はダッシュボード用のデータ
type DashboardData struct {
	TotalVisits     int
	DomainPathStats []DomainPathStats
	RecentVisits    []HistoryVisit
	MaxDomainHits   int
}

// handleDashboard はダッシュボードページを表示
func (s *WebServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	total, err := getTotalVisits(s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filter := SearchFilter{IgnoreDomains: s.ignoreDomains}
	domainPathStats, err := getDomainPathStats(s.db, DefaultDomainLimit, DefaultPathLimit, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	recentVisits, err := getRecentVisits(s.db, WebDashboardRecentVisits, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	maxHits := 0
	if len(domainPathStats) > 0 {
		maxHits = domainPathStats[0].TotalCount
	}

	data := DashboardData{
		TotalVisits:     total,
		DomainPathStats: domainPathStats,
		RecentVisits:    recentVisits,
		MaxDomainHits:   maxHits,
	}

	if err := s.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// HistoryPageData は履歴ページ用のデータ
type HistoryPageData struct {
	Visits      []HistoryVisit
	CurrentPage int
	TotalPages  int
	HasPrev     bool
	HasNext     bool
	PrevPage    int
	NextPage    int
	// フィルタ
	Search  string
	Domain  string
	From    string
	To      string
	Domains []string
}

// handleHistory は履歴一覧ページを表示
func (s *WebServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	// フィルタ条件を取得
	filter := SearchFilter{IgnoreDomains: s.ignoreDomains}
	searchQuery := r.URL.Query().Get("search")
	domainQuery := r.URL.Query().Get("domain")
	fromQuery := r.URL.Query().Get("from")
	toQuery := r.URL.Query().Get("to")

	filter.Keyword = searchQuery
	filter.Domain = domainQuery

	if fromQuery != "" {
		if t, err := time.Parse(TimeFormatDate, fromQuery); err == nil {
			filter.From = t
		}
	}
	if toQuery != "" {
		if t, err := time.Parse(TimeFormatDate, toQuery); err == nil {
			filter.To = t
		}
	}

	perPage := WebPageSize
	offset := (page - 1) * perPage

	// フィルタ付きの総件数を取得
	total, err := getFilteredVisitCount(s.db, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	totalPages := (total + perPage - 1) / perPage
	if totalPages == 0 {
		totalPages = 1
	}

	// offsetを使った取得
	visits, err := getRecentVisitsWithOffset(s.db, perPage, offset, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// ドメイン一覧を取得
	domains, err := getAllDomains(s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := HistoryPageData{
		Visits:      visits,
		CurrentPage: page,
		TotalPages:  totalPages,
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
		PrevPage:    page - 1,
		NextPage:    page + 1,
		Search:      searchQuery,
		Domain:      domainQuery,
		From:        fromQuery,
		To:          toQuery,
		Domains:     domains,
	}

	if err := s.templates.ExecuteTemplate(w, "history.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleAPIStats は統計データをJSONで返す
func (s *WebServer) handleAPIStats(w http.ResponseWriter, r *http.Request) {
	total, err := getTotalVisits(s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filter := SearchFilter{IgnoreDomains: s.ignoreDomains}
	domainStats, err := getDomainStats(s.db, DefaultDomainLimit, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hourlyStats, err := getHourlyStats(s.db, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := AnalysisResult{
		TotalVisits: total,
		DomainStats: domainStats,
		HourlyStats: hourlyStats,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleAPIHistory は履歴データをJSONで返す
func (s *WebServer) handleAPIHistory(w http.ResponseWriter, r *http.Request) {
	limit := WebPageSize
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	visits, err := getRecentVisits(s.db, limit, SearchFilter{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(visits); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// getRecentVisitsWithOffset はオフセット付きで履歴を取得
func getRecentVisitsWithOffset(db *sql.DB, limit, offset int, filter SearchFilter) ([]HistoryVisit, error) {
	qb := NewQueryBuilder(historyBaseQuery).
		WithFilter(filter).
		OrderByDesc("hv.visit_time").
		Limit(limit).
		Offset(offset)

	query, args := qb.Build()
	return executeHistoryQuery(db, query, args)
}

// カウント取得用のベースクエリ
const countBaseQuery = `
	SELECT COUNT(*)
	FROM history_visits hv
	JOIN history_items hi ON hv.history_item = hi.id
	WHERE 1=1`

// getFilteredVisitCount はフィルタ条件に一致する訪問数を取得
func getFilteredVisitCount(db *sql.DB, filter SearchFilter) (int, error) {
	qb := NewQueryBuilder(countBaseQuery).WithFilter(filter)
	query, args := qb.Build()

	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("訪問数の取得に失敗: %w", err)
	}
	return count, nil
}

// getAllDomains は全てのドメインを取得
func getAllDomains(db *sql.DB) ([]string, error) {
	query := `
		SELECT DISTINCT COALESCE(domain_expansion, '') as domain
		FROM history_items
		WHERE domain_expansion IS NOT NULL AND domain_expansion != ''
		ORDER BY domain
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("ドメイン一覧の取得に失敗: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var domains []string
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			return nil, fmt.Errorf("行の読み取りに失敗: %w", err)
		}
		domains = append(domains, domain)
	}
	return domains, nil
}

// StatsPageData は統計ページ用のデータ
type StatsPageData struct {
	HourlyStats []HourlyStats
	DailyStats  []DailyStats
	DomainStats []DomainStats
	Domains     []string
	Domain      string
	Days        int
}

// handleStatsPage は統計ページを表示
func (s *WebServer) handleStatsPage(w http.ResponseWriter, r *http.Request) {
	domainQuery := r.URL.Query().Get("domain")
	days := WebDefaultDays
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}

	filter := SearchFilter{Domain: domainQuery, IgnoreDomains: s.ignoreDomains}

	hourlyStats, err := getHourlyStats(s.db, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dailyStats, err := getDailyStats(s.db, days, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	domainStats, err := getDomainStats(s.db, DefaultDomainLimit, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	domains, err := getAllDomains(s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := StatsPageData{
		HourlyStats: hourlyStats,
		DailyStats:  dailyStats,
		DomainStats: domainStats,
		Domains:     domains,
		Domain:      domainQuery,
		Days:        days,
	}

	if err := s.templates.ExecuteTemplate(w, "stats.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleAPIStatsHourly は時間帯別統計をJSONで返す
func (s *WebServer) handleAPIStatsHourly(w http.ResponseWriter, r *http.Request) {
	domainQuery := r.URL.Query().Get("domain")
	filter := SearchFilter{Domain: domainQuery, IgnoreDomains: s.ignoreDomains}

	hourlyStats, err := getHourlyStats(s.db, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(hourlyStats); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleAPIStatsDaily は日別統計をJSONで返す
func (s *WebServer) handleAPIStatsDaily(w http.ResponseWriter, r *http.Request) {
	domainQuery := r.URL.Query().Get("domain")
	days := WebDefaultDays
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}

	filter := SearchFilter{Domain: domainQuery, IgnoreDomains: s.ignoreDomains}
	dailyStats, err := getDailyStats(s.db, days, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(dailyStats); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleAPIDomains はドメイン一覧をJSONで返す
func (s *WebServer) handleAPIDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := getAllDomains(s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(domains); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
