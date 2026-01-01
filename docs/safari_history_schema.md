# Safari History Database Schema

Safari の履歴は SQLite データベースとして保存されている。

**ファイルパス:** `~/Library/Safari/History.db`

## 主要テーブル

### history_items

URL ごとの集約情報を保持するテーブル。

| カラム名 | 型 | 説明 |
|---------|-----|------|
| id | INTEGER | 主キー（AUTO INCREMENT） |
| url | TEXT | URL（NOT NULL, UNIQUE） |
| domain_expansion | TEXT | ドメイン名（例: youtube, moneyforward） |
| visit_count | INTEGER | 訪問回数 |
| daily_visit_counts | BLOB | 日別訪問数（バイナリ） |
| weekly_visit_counts | BLOB | 週別訪問数（バイナリ） |
| autocomplete_triggers | BLOB | オートコンプリート用データ |
| should_recompute_derived_visit_counts | INTEGER | 再計算フラグ |
| visit_count_score | INTEGER | 訪問スコア |
| status_code | INTEGER | HTTPステータスコード（デフォルト: 0） |

### history_visits

個別の訪問記録を保持するテーブル。

| カラム名 | 型 | 説明 |
|---------|-----|------|
| id | INTEGER | 主キー（AUTO INCREMENT） |
| history_item | INTEGER | history_items.id への外部キー |
| visit_time | REAL | 訪問時刻（Core Data timestamp） |
| title | TEXT | ページタイトル |
| load_successful | BOOLEAN | 読み込み成功フラグ（デフォルト: 1） |
| http_non_get | BOOLEAN | GET以外のHTTPメソッド（デフォルト: 0） |
| synthesized | BOOLEAN | 合成された訪問か（デフォルト: 0） |
| redirect_source | INTEGER | リダイレクト元の history_visits.id |
| redirect_destination | INTEGER | リダイレクト先の history_visits.id |
| origin | INTEGER | 起点（デフォルト: 0） |
| generation | INTEGER | 世代（デフォルト: 0） |
| attributes | INTEGER | 属性フラグ（デフォルト: 0） |
| score | INTEGER | スコア（デフォルト: 0） |

## 時刻形式

`visit_time` は **Core Data timestamp** 形式で保存されている。

- 基準日: 2001年1月1日 00:00:00 UTC
- 単位: 秒（小数点以下はミリ秒）

### 変換例（Go）

```go
var coreDataEpoch = time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

func convertCoreDataTimestamp(timestamp float64) time.Time {
    return coreDataEpoch.Add(time.Duration(timestamp * float64(time.Second)))
}
```

## よく使うクエリ

### 訪問履歴を取得（URL・タイトル・日時）

```sql
SELECT
    hi.url,
    hv.title,
    hv.visit_time
FROM history_visits hv
JOIN history_items hi ON hv.history_item = hi.id
ORDER BY hv.visit_time DESC
LIMIT 10;
```

### ドメイン別の訪問回数

```sql
SELECT
    domain_expansion,
    SUM(visit_count) as total_visits
FROM history_items
WHERE domain_expansion IS NOT NULL
GROUP BY domain_expansion
ORDER BY total_visits DESC
LIMIT 10;
```
