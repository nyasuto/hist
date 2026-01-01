# hist

macOS Safari の履歴を分析する CLI ツール

## インストール

```bash
git clone https://github.com/nyasuto/hist.git
cd hist
make build
```

## 使い方

### 基本的な使い方

```bash
# 最近の訪問履歴を表示（デフォルト）
./hist

# ドメイン別訪問統計
./hist -domain-stats

# 時間帯別訪問統計
./hist -hourly

# 日別訪問統計
./hist -daily

# 全ての分析結果を表示
./hist -all

# JSON形式で出力
./hist -all -json
```

### インタラクティブモード

TUIベースの履歴ブラウザを起動します。

```bash
./hist -interactive
# または
./hist -i
```

**操作方法:**
- `↑`/`↓` または `j`/`k`: 履歴をナビゲート
- `Enter`: 選択した履歴の詳細を表示
- `/`: 検索モード（URL・タイトルで検索）
- `Esc`: 検索をクリア / 詳細表示を閉じる
- `r`: 履歴をリロード
- `q` または `Ctrl+C`: 終了

### 検索・フィルタ

```bash
# キーワードで検索（URL・タイトル）
./hist -search "github"

# 特定のドメインでフィルタ
./hist -domain youtube

# 日付範囲でフィルタ
./hist -from 2024-01-01 -to 2024-01-31

# 組み合わせ
./hist -domain google -from 2024-12-01 -search "maps"
```

### エクスポート

```bash
# CSV形式で出力
./hist -csv

# TSV形式で出力
./hist -tsv

# ファイルに保存
./hist -csv -output history.csv
./hist -all -csv -output full_report.csv
```

## オプション一覧

### 表示オプション

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `-history` | true | 履歴一覧を表示 |
| `-domain-stats` | false | ドメイン別統計を表示 |
| `-hourly` | false | 時間帯別統計を表示 |
| `-daily` | false | 日別統計を表示 |
| `-all` | false | 全ての分析結果を表示 |
| `-interactive`, `-i` | false | インタラクティブモードで起動 |

### 出力形式

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `-json` | false | JSON形式で出力 |
| `-csv` | false | CSV形式で出力 |
| `-tsv` | false | TSV形式で出力 |
| `-output` | - | 出力ファイルパス |

### 検索・フィルタ

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `-search` | - | キーワード検索（URL・タイトル） |
| `-domain` | - | ドメインでフィルタ |
| `-from` | - | 開始日（YYYY-MM-DD） |
| `-to` | - | 終了日（YYYY-MM-DD） |

### その他

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `-limit` | 20 | 履歴表示件数 |
| `-domains` | 10 | ドメイン統計表示件数 |
| `-days` | 7 | 日別統計の対象日数 |

## 出力例

```
📊 Safari 履歴分析結果
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
総訪問数: 19060

🌐 ドメイン別訪問数 (Top 10)
─────────────────────────────────────────
  youtube              ██████████████████ 3425
  google               █████████████████ 3401
  x                    ███ 677
  github               █ 190
```

## 必要条件

- macOS
- Go 1.20+
- Safari（履歴データ）

## ライセンス

MIT
