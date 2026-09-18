package common

import (
	"fmt"
	"regexp"
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
	q = query

	var wheres []string

	if qc.HasBanned && qo.WithBanned == false {
		wheres = append(wheres, qo.getColumn("banned_at", "IS NULL", qc))
	}
	if qc.HasSpammed && qo.WithSpammed == false {
		wheres = append(wheres, qo.getColumn("spammed_at", "IS NULL", qc))
	}
	if qc.HasDeleted && qo.WithDeleted == false {
		wheres = append(wheres, qo.getColumn("deleted_at", "IS NULL", qc))
	}

	if len(wheres) > 0 {
		if !hasTopLevelWhere(query) {
			q = fmt.Sprintf("%s WHERE ", query)
		} else {
			q = fmt.Sprintf("%s AND ", query)
		}

		q = fmt.Sprintf("%s %s", q, strings.Join(wheres, " AND "))
	}

	if qo.OrderBy != "" && validIdentifier.MatchString(qo.OrderBy) {
		order := qo.Order
		if order != Ascending && order != Descending {
			order = Descending
		}
		q = fmt.Sprintf("%s ORDER BY %s %s", q, qo.OrderBy, string(order))
	}

	if qo.Limit > 0 {
		q = fmt.Sprintf("%s LIMIT %d", q, qo.Limit)
		if qo.Page > 0 {
			q = fmt.Sprintf("%s OFFSET %d", q, int(((qo.Page-1)*qo.Limit)-qo.OffAdjust))
		}
	}

	return q
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

func (qo QueryOptions) getColumn(
	name string,
	val string,
	qc QueryCapabilities,
) (column string) {
	column = name
	if qc.Table != "" && validIdentifier.MatchString(qc.Table) {
		column = qc.Table + "." + name
	}
	column += " " + val

	return column
}
