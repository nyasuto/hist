package main

import (
	"database/sql"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// インタラクティブモードのスタイル定義
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57"))

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	domainStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	searchPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Bold(true)
)

// interactiveModel はインタラクティブモードのモデル
type interactiveModel struct {
	db           *sql.DB
	visits       []HistoryVisit
	cursor       int
	pageSize     int
	totalVisits  int
	filter       SearchFilter
	searchMode   bool
	searchInput  string
	showDetail   bool
	detailVisit  *HistoryVisit
	err          error
	windowHeight int
	windowWidth  int
}

// newInteractiveModel は新しいインタラクティブモデルを作成
func newInteractiveModel(db *sql.DB) interactiveModel {
	return interactiveModel{
		db:       db,
		pageSize: 15,
		filter:   SearchFilter{},
	}
}

// loadVisits は履歴を読み込む
func (m *interactiveModel) loadVisits() tea.Cmd {
	return func() tea.Msg {
		visits, err := getRecentVisits(m.db, m.pageSize, m.filter)
		if err != nil {
			return errMsg{err}
		}
		total, err := getTotalVisits(m.db)
		if err != nil {
			return errMsg{err}
		}
		return visitsLoadedMsg{visits: visits, total: total}
	}
}

// メッセージ型
type visitsLoadedMsg struct {
	visits []HistoryVisit
	total  int
}

type errMsg struct {
	err error
}

// Init は初期化コマンドを返す
func (m interactiveModel) Init() tea.Cmd {
	return m.loadVisits()
}

// Update はメッセージを処理してモデルを更新
func (m interactiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowHeight = msg.Height
		m.windowWidth = msg.Width
		// ウィンドウサイズに応じてページサイズを調整
		m.pageSize = max(5, msg.Height-10)
		return m, m.loadVisits()

	case visitsLoadedMsg:
		m.visits = msg.visits
		m.totalVisits = msg.total
		m.err = nil
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		// 検索モード中のキー処理
		if m.searchMode {
			return m.handleSearchInput(msg)
		}

		// 詳細表示モード中
		if m.showDetail {
			switch msg.String() {
			case "esc", "q", "enter":
				m.showDetail = false
				m.detailVisit = nil
			}
			return m, nil
		}

		// 通常モードのキー処理
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.visits)-1 {
				m.cursor++
			}

		case "enter":
			if len(m.visits) > 0 && m.cursor < len(m.visits) {
				m.showDetail = true
				m.detailVisit = &m.visits[m.cursor]
			}

		case "/":
			m.searchMode = true
			m.searchInput = m.filter.Keyword

		case "esc":
			// 検索をクリア
			if m.filter.Keyword != "" {
				m.filter.Keyword = ""
				m.cursor = 0
				return m, m.loadVisits()
			}

		case "r":
			// リロード
			return m, m.loadVisits()
		}
	}

	return m, nil
}

// handleSearchInput は検索モードのキー入力を処理
func (m interactiveModel) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.searchMode = false
		m.filter.Keyword = m.searchInput
		m.cursor = 0
		return m, m.loadVisits()

	case "esc":
		m.searchMode = false
		m.searchInput = m.filter.Keyword
		return m, nil

	case "backspace":
		if len(m.searchInput) > 0 {
			m.searchInput = m.searchInput[:len(m.searchInput)-1]
		}

	case "ctrl+c":
		return m, tea.Quit

	default:
		// 通常の文字入力
		if len(msg.String()) == 1 {
			m.searchInput += msg.String()
		}
	}

	return m, nil
}

// View はUIを描画
func (m interactiveModel) View() string {
	var b strings.Builder

	// タイトル
	b.WriteString(titleStyle.Render("Safari 履歴ブラウザ"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(50, m.windowWidth)))
	b.WriteString("\n\n")

	// エラー表示
	if m.err != nil {
		b.WriteString(fmt.Sprintf("エラー: %v\n", m.err))
		return b.String()
	}

	// 詳細表示モード
	if m.showDetail && m.detailVisit != nil {
		return m.renderDetail()
	}

	// 検索モード表示
	if m.searchMode {
		b.WriteString(searchPromptStyle.Render("検索: "))
		b.WriteString(m.searchInput)
		b.WriteString("_\n\n")
	} else if m.filter.Keyword != "" {
		b.WriteString(fmt.Sprintf("検索中: %q (Escでクリア)\n\n", m.filter.Keyword))
	}

	// 履歴一覧
	if len(m.visits) == 0 {
		b.WriteString("履歴がありません\n")
	} else {
		for i, v := range m.visits {
			cursor := "  "
			if m.cursor == i {
				cursor = "> "
			}

			title := v.Title
			if title == "" {
				title = "(タイトルなし)"
			}
			// タイトルを切り詰め
			maxTitleLen := min(60, m.windowWidth-20)
			if len(title) > maxTitleLen {
				title = title[:maxTitleLen-3] + "..."
			}

			line := fmt.Sprintf("%s%s  %s",
				cursor,
				v.VisitTime.Format("01/02 15:04"),
				title,
			)

			if m.cursor == i {
				b.WriteString(selectedStyle.Render(line))
			} else {
				b.WriteString(normalStyle.Render(line))
			}
			b.WriteString("\n")

			// ドメイン表示
			if v.Domain != "" {
				domainLine := fmt.Sprintf("             %s", v.Domain)
				b.WriteString(domainStyle.Render(domainLine))
				b.WriteString("\n")
			}
		}
	}

	// フッター
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(50, m.windowWidth)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("総訪問数: %d\n", m.totalVisits))
	b.WriteString(helpStyle.Render("↑/↓:移動  Enter:詳細  /:検索  r:更新  q:終了"))
	b.WriteString("\n")

	return b.String()
}

// renderDetail は詳細画面を描画
func (m interactiveModel) renderDetail() string {
	var b strings.Builder
	v := m.detailVisit

	b.WriteString(titleStyle.Render("履歴詳細"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(50, m.windowWidth)))
	b.WriteString("\n\n")

	title := v.Title
	if title == "" {
		title = "(タイトルなし)"
	}

	b.WriteString(fmt.Sprintf("タイトル: %s\n\n", title))
	b.WriteString(fmt.Sprintf("URL: %s\n\n", v.URL))
	b.WriteString(fmt.Sprintf("ドメイン: %s\n\n", v.Domain))
	b.WriteString(fmt.Sprintf("訪問日時: %s\n\n", v.VisitTime.Format("2006-01-02 15:04:05")))

	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(50, m.windowWidth)))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Enter/Esc/q:戻る"))
	b.WriteString("\n")

	return b.String()
}

// runInteractiveMode はインタラクティブモードを実行
func runInteractiveMode(db *sql.DB) error {
	m := newInteractiveModel(db)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
