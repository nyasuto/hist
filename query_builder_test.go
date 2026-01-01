package main

import (
	"testing"
	"time"
)

func TestNewQueryBuilder(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	qb := NewQueryBuilder(baseQuery)

	query, args := qb.Build()
	if query != baseQuery {
		t.Errorf("期待値 %q, 実際 %q", baseQuery, query)
	}
	if len(args) != 0 {
		t.Errorf("期待値 0個の引数, 実際 %d個", len(args))
	}
}

func TestQueryBuilderWithKeyword(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	qb := NewQueryBuilder(baseQuery).WithKeyword("test")

	query, args := qb.Build()
	expectedQuery := baseQuery + ` AND (hi.url LIKE ? OR hv.title LIKE ?)`
	if query != expectedQuery {
		t.Errorf("期待値 %q, 実際 %q", expectedQuery, query)
	}
	if len(args) != 2 {
		t.Fatalf("期待値 2個の引数, 実際 %d個", len(args))
	}
	if args[0] != "%test%" || args[1] != "%test%" {
		t.Errorf("期待値 %%test%%, 実際 %v", args)
	}
}

func TestQueryBuilderWithEmptyKeyword(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	qb := NewQueryBuilder(baseQuery).WithKeyword("")

	query, args := qb.Build()
	if query != baseQuery {
		t.Errorf("空のキーワードでクエリが変更された: %q", query)
	}
	if len(args) != 0 {
		t.Errorf("空のキーワードで引数が追加された: %v", args)
	}
}

func TestQueryBuilderWithDomain(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	qb := NewQueryBuilder(baseQuery).WithDomain("example.com")

	query, args := qb.Build()
	expectedQuery := baseQuery + ` AND hi.domain_expansion = ?`
	if query != expectedQuery {
		t.Errorf("期待値 %q, 実際 %q", expectedQuery, query)
	}
	if len(args) != 1 || args[0] != "example.com" {
		t.Errorf("期待値 [example.com], 実際 %v", args)
	}
}

func TestQueryBuilderWithEmptyDomain(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	qb := NewQueryBuilder(baseQuery).WithDomain("")

	query, args := qb.Build()
	if query != baseQuery {
		t.Errorf("空のドメインでクエリが変更された: %q", query)
	}
	if len(args) != 0 {
		t.Errorf("空のドメインで引数が追加された: %v", args)
	}
}

func TestQueryBuilderWithDateRange(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	qb := NewQueryBuilder(baseQuery).WithDateRange(from, to)

	query, args := qb.Build()
	expectedQuery := baseQuery + ` AND hv.visit_time >= ?` + ` AND hv.visit_time <= ?`
	if query != expectedQuery {
		t.Errorf("期待値 %q, 実際 %q", expectedQuery, query)
	}
	if len(args) != 2 {
		t.Fatalf("期待値 2個の引数, 実際 %d個", len(args))
	}
}

func TestQueryBuilderWithFromDateOnly(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	zeroTime := time.Time{}

	qb := NewQueryBuilder(baseQuery).WithDateRange(from, zeroTime)

	query, args := qb.Build()
	expectedQuery := baseQuery + ` AND hv.visit_time >= ?`
	if query != expectedQuery {
		t.Errorf("期待値 %q, 実際 %q", expectedQuery, query)
	}
	if len(args) != 1 {
		t.Fatalf("期待値 1個の引数, 実際 %d個", len(args))
	}
}

func TestQueryBuilderWithToDateOnly(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	zeroTime := time.Time{}
	to := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	qb := NewQueryBuilder(baseQuery).WithDateRange(zeroTime, to)

	query, args := qb.Build()
	expectedQuery := baseQuery + ` AND hv.visit_time <= ?`
	if query != expectedQuery {
		t.Errorf("期待値 %q, 実際 %q", expectedQuery, query)
	}
	if len(args) != 1 {
		t.Fatalf("期待値 1個の引数, 実際 %d個", len(args))
	}
}

func TestQueryBuilderWithFilter(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	filter := SearchFilter{
		Keyword: "test",
		Domain:  "example.com",
		From:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		To:      time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}

	qb := NewQueryBuilder(baseQuery).WithFilter(filter)

	query, args := qb.Build()
	if len(args) != 5 { // 2 (keyword) + 1 (domain) + 2 (date range)
		t.Errorf("期待値 5個の引数, 実際 %d個", len(args))
	}
	// キーワード条件が含まれているか
	if query == baseQuery {
		t.Error("フィルタが適用されていない")
	}
}

func TestQueryBuilderOrderByDesc(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	qb := NewQueryBuilder(baseQuery).OrderByDesc("visit_time")

	query, _ := qb.Build()
	expectedQuery := baseQuery + ` ORDER BY visit_time DESC`
	if query != expectedQuery {
		t.Errorf("期待値 %q, 実際 %q", expectedQuery, query)
	}
}

func TestQueryBuilderLimit(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	qb := NewQueryBuilder(baseQuery).Limit(10)

	query, args := qb.Build()
	expectedQuery := baseQuery + ` LIMIT ?`
	if query != expectedQuery {
		t.Errorf("期待値 %q, 実際 %q", expectedQuery, query)
	}
	if len(args) != 1 || args[0] != 10 {
		t.Errorf("期待値 [10], 実際 %v", args)
	}
}

func TestQueryBuilderOffset(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	qb := NewQueryBuilder(baseQuery).Offset(20)

	query, args := qb.Build()
	expectedQuery := baseQuery + ` OFFSET ?`
	if query != expectedQuery {
		t.Errorf("期待値 %q, 実際 %q", expectedQuery, query)
	}
	if len(args) != 1 || args[0] != 20 {
		t.Errorf("期待値 [20], 実際 %v", args)
	}
}

func TestQueryBuilderChaining(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	qb := NewQueryBuilder(baseQuery).
		WithKeyword("test").
		WithDomain("example.com").
		OrderByDesc("visit_time").
		Limit(10).
		Offset(20)

	query, args := qb.Build()

	// すべての条件が含まれているか確認
	expectedParts := []string{
		"AND (hi.url LIKE ? OR hv.title LIKE ?)",
		"AND hi.domain_expansion = ?",
		"ORDER BY visit_time DESC",
		"LIMIT ?",
		"OFFSET ?",
	}
	for _, part := range expectedParts {
		if !containsString(query, part) {
			t.Errorf("クエリに %q が含まれていない: %q", part, query)
		}
	}

	// 引数の数を確認 (keyword: 2, domain: 1, limit: 1, offset: 1)
	if len(args) != 5 {
		t.Errorf("期待値 5個の引数, 実際 %d個: %v", len(args), args)
	}
}

func TestQueryBuilderArgs(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	qb := NewQueryBuilder(baseQuery).WithKeyword("test").WithDomain("example.com")

	args := qb.Args()
	if len(args) != 3 { // 2 (keyword) + 1 (domain)
		t.Errorf("期待値 3個の引数, 実際 %d個", len(args))
	}
}

func TestQueryBuilderWithIgnoreDomains(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	qb := NewQueryBuilder(baseQuery).WithIgnoreDomains([]string{"youtube.com", "google.com"})

	query, args := qb.Build()
	// COALESCE、サブドメイン除外、URL除外が含まれるか確認
	expectedParts := []string{
		"AND COALESCE(hi.domain_expansion, '') != ?",
		"AND COALESCE(hi.domain_expansion, '') NOT LIKE ?",
		"AND hi.url NOT LIKE ?",
	}
	for _, part := range expectedParts {
		if !containsString(query, part) {
			t.Errorf("クエリに %q が含まれていない: %q", part, query)
		}
	}
	// 各ドメインに対して4つの引数（完全一致 + サブドメインLIKE + URL2パターン）
	if len(args) != 8 {
		t.Errorf("期待値 8個の引数, 実際 %d個: %v", len(args), args)
	}
	// youtube.com用の引数
	if args[0] != "youtube.com" || args[1] != "%.youtube.com" {
		t.Errorf("期待値 [youtube.com, %%.youtube.com], 実際 %v", args[:2])
	}
	if args[2] != "%://youtube.com.%" || args[3] != "%://%.youtube.com.%" {
		t.Errorf("期待値 [%%://youtube.com.%%, %%://%%.youtube.com.%%], 実際 %v", args[2:4])
	}
}

func TestQueryBuilderWithEmptyIgnoreDomains(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	qb := NewQueryBuilder(baseQuery).WithIgnoreDomains([]string{})

	query, args := qb.Build()
	if query != baseQuery {
		t.Errorf("空のイグノアリストでクエリが変更された: %q", query)
	}
	if len(args) != 0 {
		t.Errorf("空のイグノアリストで引数が追加された: %v", args)
	}
}

func TestQueryBuilderWithFilterIncludesIgnoreDomains(t *testing.T) {
	baseQuery := "SELECT * FROM test WHERE 1=1"
	filter := SearchFilter{
		Keyword:       "test",
		IgnoreDomains: []string{"youtube.com"},
	}
	qb := NewQueryBuilder(baseQuery).WithFilter(filter)

	query, args := qb.Build()
	if !containsString(query, "AND COALESCE(hi.domain_expansion, '') != ?") {
		t.Errorf("フィルタにイグノアドメインが適用されていない: %q", query)
	}
	if !containsString(query, "AND hi.url NOT LIKE ?") {
		t.Errorf("フィルタにURL除外が適用されていない: %q", query)
	}
	// keyword: 2, ignoreDomains: 4 (完全一致 + サブドメイン + URL2パターン)
	if len(args) != 6 {
		t.Errorf("期待値 6個の引数, 実際 %d個", len(args))
	}
}

// containsString はsがsubstrを含むかをチェック
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
