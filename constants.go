package main

// データベース関連の定数
const (
	// SafariHistoryPath はSafari履歴DBの相対パス（ホームディレクトリからの）
	SafariHistoryPath = "Library/Safari/History.db"
	// SQLiteDriver はSQLiteのドライバ名
	SQLiteDriver = "sqlite3"
	// SQLiteReadOnlyMode は読み取り専用モードのクエリパラメータ
	SQLiteReadOnlyMode = "?mode=ro"
)

// CLI デフォルト値
const (
	// DefaultHistoryLimit は履歴表示のデフォルト件数
	DefaultHistoryLimit = 20
	// DefaultDomainLimit はドメイン統計のデフォルト表示件数
	DefaultDomainLimit = 10
	// DefaultPathLimit は各ドメイン内で表示するパス数
	DefaultPathLimit = 5
	// DefaultDailyDays は日別統計のデフォルト日数
	DefaultDailyDays = 7
	// DefaultWebPort はWebサーバーのデフォルトポート
	DefaultWebPort = 8080
)

// Web UI 関連の定数
const (
	// WebPageSize はWeb UIでの1ページあたりの表示件数
	WebPageSize = 50
	// WebDashboardRecentVisits はダッシュボードの最近の訪問表示件数
	WebDashboardRecentVisits = 5
	// WebDefaultDays は統計ページのデフォルト日数
	WebDefaultDays = 30
)

// インタラクティブモード関連の定数
const (
	// DefaultPageSize はインタラクティブモードのデフォルトページサイズ
	DefaultPageSize = 15
	// MinPageSize はインタラクティブモードの最小ページサイズ
	MinPageSize = 5
	// MaxTitleLength はタイトルの最大表示長
	MaxTitleLength = 60
	// SeparatorWidth はセパレータの幅
	SeparatorWidth = 50
	// TitleTruncateLength はテキスト出力でのタイトル切り詰め長
	TitleTruncateLength = 50
)

// UI表示関連の定数
const (
	// BarChartWidth はバーチャートの幅
	BarChartWidth = 20
)

// 時刻フォーマット
const (
	// TimeFormatFull は完全な日時フォーマット（秒まで）
	TimeFormatFull = "2006-01-02 15:04:05"
	// TimeFormatDate は日付のみのフォーマット
	TimeFormatDate = "2006-01-02"
	// TimeFormatDateTime は日時フォーマット（分まで）
	TimeFormatDateTime = "2006-01-02 15:04"
	// TimeFormatShort は短い日時フォーマット
	TimeFormatShort = "01/02 15:04"
)
