package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestNewInteractiveModel はモデル初期化のテスト
func TestNewInteractiveModel(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	m := newInteractiveModel(db)

	if m.db != db {
		t.Error("dbが正しく設定されていない")
	}
	if m.pageSize != 15 {
		t.Errorf("pageSize = %d, want 15", m.pageSize)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
	if m.searchMode {
		t.Error("searchModeが初期状態でtrueになっている")
	}
	if m.showDetail {
		t.Error("showDetailが初期状態でtrueになっている")
	}
}

// TestInteractiveModelInit は初期化コマンドのテスト
func TestInteractiveModelInit(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	insertTestData(t, db)

	m := newInteractiveModel(db)
	cmd := m.Init()

	if cmd == nil {
		t.Error("Init()がnilを返した")
	}
}

// TestInteractiveModelUpdateKeyNavigation はキーボードナビゲーションのテスト
func TestInteractiveModelUpdateKeyNavigation(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	insertTestData(t, db)

	m := newInteractiveModel(db)
	// 手動でvisitsを設定
	m.visits = []HistoryVisit{
		{Title: "Test 1"},
		{Title: "Test 2"},
		{Title: "Test 3"},
	}

	// 下キーでカーソル移動
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(interactiveModel)
	if m.cursor != 1 {
		t.Errorf("down後のcursor = %d, want 1", m.cursor)
	}

	// さらに下
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(interactiveModel)
	if m.cursor != 2 {
		t.Errorf("2回目down後のcursor = %d, want 2", m.cursor)
	}

	// 最下部でさらに下を押しても動かない
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(interactiveModel)
	if m.cursor != 2 {
		t.Errorf("最下部でdown後のcursor = %d, want 2", m.cursor)
	}

	// 上キーでカーソル移動
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(interactiveModel)
	if m.cursor != 1 {
		t.Errorf("up後のcursor = %d, want 1", m.cursor)
	}

	// 最上部まで移動
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(interactiveModel)
	if m.cursor != 0 {
		t.Errorf("最上部でのcursor = %d, want 0", m.cursor)
	}

	// 最上部でさらに上を押しても動かない
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(interactiveModel)
	if m.cursor != 0 {
		t.Errorf("最上部でup後のcursor = %d, want 0", m.cursor)
	}
}

// TestInteractiveModelUpdateSearchMode は検索モードのテスト
func TestInteractiveModelUpdateSearchMode(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	m := newInteractiveModel(db)
	m.visits = []HistoryVisit{{Title: "Test"}}

	// / キーで検索モードに入る
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = newModel.(interactiveModel)
	if !m.searchMode {
		t.Error("/ キーで検索モードに入れていない")
	}

	// 文字を入力
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = newModel.(interactiveModel)
	if m.searchInput != "t" {
		t.Errorf("searchInput = %q, want %q", m.searchInput, "t")
	}

	// Escで検索モードを抜ける
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(interactiveModel)
	if m.searchMode {
		t.Error("Escで検索モードを抜けられていない")
	}
}

// TestInteractiveModelUpdateDetailView は詳細表示のテスト
func TestInteractiveModelUpdateDetailView(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	m := newInteractiveModel(db)
	m.visits = []HistoryVisit{
		{Title: "Test", URL: "https://example.com", Domain: "example"},
	}

	// Enterで詳細表示
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(interactiveModel)
	if !m.showDetail {
		t.Error("Enterで詳細表示に入れていない")
	}
	if m.detailVisit == nil {
		t.Error("detailVisitがnilになっている")
	}
	if m.detailVisit.Title != "Test" {
		t.Errorf("detailVisit.Title = %q, want %q", m.detailVisit.Title, "Test")
	}

	// Escで戻る
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(interactiveModel)
	if m.showDetail {
		t.Error("Escで詳細表示を抜けられていない")
	}
}

// TestInteractiveModelView はView関数のテスト
func TestInteractiveModelView(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	m := newInteractiveModel(db)
	m.windowWidth = 80
	m.windowHeight = 24
	m.visits = []HistoryVisit{
		{Title: "Test Title", Domain: "example", URL: "https://example.com"},
	}
	m.totalVisits = 100

	view := m.View()

	// タイトルが含まれているか
	if !contains(view, "Safari 履歴ブラウザ") {
		t.Error("Viewにタイトルが含まれていない")
	}

	// 訪問数が含まれているか
	if !contains(view, "100") {
		t.Error("Viewに総訪問数が含まれていない")
	}

	// ヘルプが含まれているか
	if !contains(view, "q:終了") {
		t.Error("Viewにヘルプが含まれていない")
	}
}

// TestInteractiveModelViewDetail は詳細画面のテスト
func TestInteractiveModelViewDetail(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	m := newInteractiveModel(db)
	m.windowWidth = 80
	m.windowHeight = 24
	m.showDetail = true
	m.detailVisit = &HistoryVisit{
		Title:  "Test Title",
		Domain: "example",
		URL:    "https://example.com",
	}

	view := m.View()

	// 詳細タイトルが含まれているか
	if !contains(view, "履歴詳細") {
		t.Error("詳細Viewにタイトルが含まれていない")
	}

	// URLが含まれているか
	if !contains(view, "https://example.com") {
		t.Error("詳細ViewにURLが含まれていない")
	}
}

// TestInteractiveModelViewSearch は検索モードの表示テスト
func TestInteractiveModelViewSearch(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	m := newInteractiveModel(db)
	m.windowWidth = 80
	m.windowHeight = 24
	m.searchMode = true
	m.searchInput = "test"
	m.visits = []HistoryVisit{}

	view := m.View()

	// 検索プロンプトが含まれているか
	if !contains(view, "検索:") {
		t.Error("検索Viewにプロンプトが含まれていない")
	}
}

// TestVisitsLoadedMsg はメッセージ処理のテスト
func TestVisitsLoadedMsg(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	m := newInteractiveModel(db)

	visits := []HistoryVisit{
		{Title: "Test 1"},
		{Title: "Test 2"},
	}

	newModel, _ := m.Update(visitsLoadedMsg{visits: visits, total: 50})
	m = newModel.(interactiveModel)

	if len(m.visits) != 2 {
		t.Errorf("visits length = %d, want 2", len(m.visits))
	}
	if m.totalVisits != 50 {
		t.Errorf("totalVisits = %d, want 50", m.totalVisits)
	}
}

// TestErrMsg はエラーメッセージ処理のテスト
func TestErrMsg(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	m := newInteractiveModel(db)

	testErr := errMsg{err: nil}
	newModel, _ := m.Update(testErr)
	m = newModel.(interactiveModel)

	if m.err != nil {
		t.Error("err should be nil")
	}
}

// TestWindowSizeMsg はウィンドウサイズメッセージのテスト
func TestWindowSizeMsg(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	m := newInteractiveModel(db)

	newModel, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = newModel.(interactiveModel)

	if m.windowWidth != 100 {
		t.Errorf("windowWidth = %d, want 100", m.windowWidth)
	}
	if m.windowHeight != 30 {
		t.Errorf("windowHeight = %d, want 30", m.windowHeight)
	}
	// pageSizeはウィンドウサイズに応じて調整される
	expectedPageSize := 30 - 10 // height - 10
	if m.pageSize != expectedPageSize {
		t.Errorf("pageSize = %d, want %d", m.pageSize, expectedPageSize)
	}
	if cmd == nil {
		t.Error("WindowSizeMsgでloadVisitsが呼ばれていない")
	}
}
