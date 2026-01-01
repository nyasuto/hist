package main

import (
	"strings"
	"time"
)

// QueryBuilder はSQLクエリのWHERE句を動的に構築するビルダー
type QueryBuilder struct {
	baseQuery string
	where     strings.Builder
	args      []interface{}
}

// NewQueryBuilder は新しいQueryBuilderを作成
func NewQueryBuilder(baseQuery string) *QueryBuilder {
	return &QueryBuilder{
		baseQuery: baseQuery,
		args:      []interface{}{},
	}
}

// WithKeyword はキーワード検索条件を追加（URL・タイトルの部分一致）
func (qb *QueryBuilder) WithKeyword(keyword string) *QueryBuilder {
	if keyword != "" {
		qb.where.WriteString(` AND (hi.url LIKE ? OR hv.title LIKE ?)`)
		likePattern := "%" + keyword + "%"
		qb.args = append(qb.args, likePattern, likePattern)
	}
	return qb
}

// WithDomain はドメインフィルタ条件を追加
func (qb *QueryBuilder) WithDomain(domain string) *QueryBuilder {
	if domain != "" {
		qb.where.WriteString(` AND hi.domain_expansion = ?`)
		qb.args = append(qb.args, domain)
	}
	return qb
}

// WithIgnoreDomains は除外ドメイン条件を追加
// サブドメインも含めて除外（例: "google" → "google", "accounts.google", "docs.google" 等を除外）
// domain_expansionがNULL/空の場合はURLからドメインを判定
func (qb *QueryBuilder) WithIgnoreDomains(domains []string) *QueryBuilder {
	for _, d := range domains {
		if d != "" {
			// domain_expansionがNULL/空の場合はURL自体でチェック
			// NULLの場合: domain_expansion != 'x' は NULL（UNKNOWN）になるため、
			// COALESCE で空文字列に変換してから比較
			qb.where.WriteString(` AND COALESCE(hi.domain_expansion, '') != ?`)
			qb.where.WriteString(` AND COALESCE(hi.domain_expansion, '') NOT LIKE ?`)
			// URL自体もチェック（domain_expansionがNULLの場合のフォールバック）
			// ドメイン部分にマッチ: ://domain. または ://domain/ または ://sub.domain.
			qb.where.WriteString(` AND hi.url NOT LIKE ?`)
			qb.where.WriteString(` AND hi.url NOT LIKE ?`)
			qb.args = append(qb.args, d, "%."+d, "%://"+d+".%", "%://%."+d+".%")
		}
	}
	return qb
}

// WithDateRange は日付範囲フィルタ条件を追加
func (qb *QueryBuilder) WithDateRange(from, to time.Time) *QueryBuilder {
	if !from.IsZero() {
		qb.where.WriteString(` AND hv.visit_time >= ?`)
		qb.args = append(qb.args, convertToTimestamp(from))
	}
	if !to.IsZero() {
		// 終了日は当日の23:59:59まで含める
		qb.where.WriteString(` AND hv.visit_time <= ?`)
		qb.args = append(qb.args, convertToTimestamp(to.Add(24*time.Hour-time.Second)))
	}
	return qb
}

// WithFilter はSearchFilter全体を適用
func (qb *QueryBuilder) WithFilter(filter SearchFilter) *QueryBuilder {
	return qb.WithKeyword(filter.Keyword).
		WithDomain(filter.Domain).
		WithDateRange(filter.From, filter.To).
		WithIgnoreDomains(filter.IgnoreDomains)
}

// OrderByDesc はORDER BY DESC句を追加
func (qb *QueryBuilder) OrderByDesc(column string) *QueryBuilder {
	qb.where.WriteString(` ORDER BY ` + column + ` DESC`)
	return qb
}

// Limit はLIMIT句を追加
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.where.WriteString(` LIMIT ?`)
	qb.args = append(qb.args, limit)
	return qb
}

// Offset はOFFSET句を追加
func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	qb.where.WriteString(` OFFSET ?`)
	qb.args = append(qb.args, offset)
	return qb
}

// Build はクエリ文字列とパラメータを返す
func (qb *QueryBuilder) Build() (string, []interface{}) {
	return qb.baseQuery + qb.where.String(), qb.args
}

// Args は現在のパラメータを返す
func (qb *QueryBuilder) Args() []interface{} {
	return qb.args
}
