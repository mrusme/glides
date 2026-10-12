package common

import (
	"regexp"
	"strconv"
	"strings"
)

type QueryOrder string

const (
	Ascending  QueryOrder = "ASC"
	Descending QueryOrder = "DESC"
)

var validIdentifier = regexp.MustCompile(`^[a-z_][a-z0-9_]*(\.[a-z_][a-z0-9_]*)?$`)

type QueryOptions struct {
	Limit       int
	Page        int
	OffAdjust   int
	Order       QueryOrder
	OrderBy     string
	WithBanned  bool
	WithSpammed bool
	WithDeleted bool
}

type QueryCapabilities struct {
	Table      string
	HasBanned  bool
	HasSpammed bool
	HasDeleted bool
}

func (qo QueryOptions) Query(
	query string,
	qc QueryCapabilities,
) (q string) {
	var b strings.Builder
	b.Grow(len(query) + 128)
	b.WriteString(query)

	joiner := " WHERE "
	if hasTopLevelWhere(query) {
		joiner = " AND "
	}

	qualified := qc.Table != "" && validIdentifier.MatchString(qc.Table)
	for _, condition := range []struct {
		has    bool
		with   bool
		column string
	}{
		{qc.HasBanned, qo.WithBanned, "banned_at"},
		{qc.HasSpammed, qo.WithSpammed, "spammed_at"},
		{qc.HasDeleted, qo.WithDeleted, "deleted_at"},
	} {
		if !condition.has || condition.with {
			continue
		}
		b.WriteString(joiner)
		if qualified {
			b.WriteString(qc.Table)
			b.WriteByte('.')
		}
		b.WriteString(condition.column)
		b.WriteString(" IS NULL")
		joiner = " AND "
	}

	if qo.OrderBy != "" && validIdentifier.MatchString(qo.OrderBy) {
		order := qo.Order
		if order != Ascending && order != Descending {
			order = Descending
		}
		b.WriteString(" ORDER BY ")
		b.WriteString(qo.OrderBy)
		b.WriteByte(' ')
		b.WriteString(string(order))
	}

	if qo.Limit > 0 {
		b.WriteString(" LIMIT ")
		b.WriteString(strconv.Itoa(qo.Limit))
		if qo.Page > 0 {
			b.WriteString(" OFFSET ")
			b.WriteString(strconv.Itoa(qo.Offset()))
		}
	}

	return b.String()
}

func (qo QueryOptions) Offset() int {
	if qo.Limit <= 0 || qo.Page <= 0 {
		return 0
	}

	return max(0, (qo.Page-1)*qo.Limit-qo.OffAdjust)
}

func hasTopLevelWhere(query string) bool {
	depth := 0
	var quote byte

	for i := 0; i < len(query); i++ {
		c := query[i]

		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}

		switch c {
		case '\'', '"':
			quote = c
		case '(':
			depth++
		case ')':
			depth--
		}

		if depth != 0 || i+5 > len(query) || !strings.EqualFold(query[i:i+5], "WHERE") {
			continue
		}
		if i > 0 && isIdentifierByte(query[i-1]) {
			continue
		}
		if i+5 < len(query) && isIdentifierByte(query[i+5]) {
			continue
		}

		return true
	}

	return false
}

func isIdentifierByte(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
