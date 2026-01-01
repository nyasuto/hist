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
		return t.Format("2006-01-02 15:04:05")
	},
	"formatDate": func(t time.Time) string {
		return t.Format("2006-01-02")
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
	db        *sql.DB
	templates *template.Template
	port      int
}

// NewWebServer は新しいWebServerを作成
func NewWebServer(db *sql.DB, port int) (*WebServer, error) {
	tmpl, err := template.New("").Funcs(templateFuncs).ParseFS(templatesFS, "web/templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("テンプレートの解析に失敗: %w", err)
	}

	return &WebServer{
		db:        db,
		templates: tmpl,
		port:      port,
	}, nil
}

// Start はWebサーバーを起動
func (s *WebServer) Start() error {
	mux := http.NewServeMux()

	// ページハンドラー
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/history", s.handleHistory)

	// APIハンドラー
	mux.HandleFunc("/api/stats", s.handleAPIStats)
	mux.HandleFunc("/api/history", s.handleAPIHistory)

	// 静的ファイル
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Web server starting at http://localhost%s", addr)
	return http.ListenAndServe(addr, mux)
}

// DashboardData はダッシュボード用のデータ
type DashboardData struct {
	TotalVisits   int
	DomainStats   []DomainStats
	RecentVisits  []HistoryVisit
	MaxDomainHits int
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

	domainStats, err := getDomainStats(s.db, 10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	recentVisits, err := getRecentVisits(s.db, 5, SearchFilter{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	maxHits := 0
	if len(domainStats) > 0 {
		maxHits = domainStats[0].VisitCount
	}

	data := DashboardData{
		TotalVisits:   total,
		DomainStats:   domainStats,
		RecentVisits:  recentVisits,
		MaxDomainHits: maxHits,
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
}

// handleHistory は履歴一覧ページを表示
func (s *WebServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	perPage := 50
	offset := (page - 1) * perPage

	// 総件数を取得
	total, err := getTotalVisits(s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	totalPages := (total + perPage - 1) / perPage
	if totalPages == 0 {
		totalPages = 1
	}

	// offsetを使った取得のためにlimitを調整
	visits, err := getRecentVisitsWithOffset(s.db, perPage, offset, SearchFilter{})
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

	domainStats, err := getDomainStats(s.db, 10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hourlyStats, err := getHourlyStats(s.db, SearchFilter{})
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
	limit := 50
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
	query := `
		SELECT
			hi.url,
			COALESCE(hv.title, '') as title,
			COALESCE(hi.domain_expansion, '') as domain,
			hv.visit_time
		FROM history_visits hv
		JOIN history_items hi ON hv.history_item = hi.id
		WHERE 1=1
	`
	args := []interface{}{}

	if filter.Keyword != "" {
		query += ` AND (hi.url LIKE ? OR hv.title LIKE ?)`
		keyword := "%" + filter.Keyword + "%"
		args = append(args, keyword, keyword)
	}

	if filter.Domain != "" {
		query += ` AND hi.domain_expansion = ?`
		args = append(args, filter.Domain)
	}

	if !filter.From.IsZero() {
		query += ` AND hv.visit_time >= ?`
		args = append(args, convertToTimestamp(filter.From))
	}
	if !filter.To.IsZero() {
		query += ` AND hv.visit_time <= ?`
		args = append(args, convertToTimestamp(filter.To.Add(24*time.Hour-time.Second)))
	}

	query += ` ORDER BY hv.visit_time DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

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
		visits = append(visits, v)
	}
	return visits, nil
}
