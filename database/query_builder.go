package database

import (
	"fmt"
	"strings"
)

// QueryBuilder provides a fluent interface for building SQL queries
type QueryBuilder struct {
	queryType      string
	table          string
	columns        []string
	values         []interface{}
	placeholders   []string
	conditions     []string
	setConditions  []string
	joins          []string
	orderBy        []string
	groupBy        []string
	having         []string
	limit          *int
	offset         *int
	args           []interface{}
	argIndex       int
	conflicts      []string
	conflictAction string
}

// Select creates a new SELECT query builder
func Select(columns ...string) *QueryBuilder {
	return &QueryBuilder{
		queryType: "SELECT",
		columns:   columns,
		args:      make([]interface{}, 0),
		argIndex:  1,
	}
}

// Insert creates a new INSERT query builder
func Insert(table string) *QueryBuilder {
	return &QueryBuilder{
		queryType: "INSERT",
		table:     table,
		args:      make([]interface{}, 0),
		argIndex:  1,
	}
}

// Update creates a new UPDATE query builder
func Update(table string) *QueryBuilder {
	return &QueryBuilder{
		queryType: "UPDATE",
		table:     table,
		args:      make([]interface{}, 0),
		argIndex:  1,
	}
}

// Delete creates a new DELETE query builder
func Delete() *QueryBuilder {
	return &QueryBuilder{
		queryType: "DELETE",
		args:      make([]interface{}, 0),
		argIndex:  1,
	}
}

// From sets the table for SELECT queries
func (qb *QueryBuilder) From(table string) *QueryBuilder {
	qb.table = table
	return qb
}

// Into sets the table for INSERT queries (alias for consistency)
func (qb *QueryBuilder) Into(table string) *QueryBuilder {
	qb.table = table
	return qb
}

// Columns sets the columns for INSERT queries
func (qb *QueryBuilder) Columns(columns ...string) *QueryBuilder {
	qb.columns = columns
	return qb
}

// Values adds values for INSERT queries
func (qb *QueryBuilder) Values(values ...interface{}) *QueryBuilder {
	qb.values = append(qb.values, values...)

	// Generate placeholders
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = fmt.Sprintf("$%d", qb.argIndex)
		qb.argIndex++
	}
	qb.placeholders = append(qb.placeholders, "("+strings.Join(placeholders, ", ")+")")
	qb.args = append(qb.args, values...)
	return qb
}

// Set adds a SET clause for UPDATE queries
func (qb *QueryBuilder) Set(column string, value interface{}) *QueryBuilder {
	condition := fmt.Sprintf("%s = $%d", column, qb.argIndex)
	qb.setConditions = append(qb.setConditions, condition)
	qb.args = append(qb.args, value)
	qb.argIndex++
	return qb
}

// SetMap adds multiple SET clauses from a map for UPDATE queries
func (qb *QueryBuilder) SetMap(values map[string]interface{}) *QueryBuilder {
	for column, value := range values {
		qb.Set(column, value)
	}
	return qb
}

// Where adds a WHERE condition
func (qb *QueryBuilder) Where(condition string, args ...interface{}) *QueryBuilder {
	// Replace ? placeholders with $n placeholders
	processedCondition := qb.processPlaceholders(condition, len(args))
	qb.conditions = append(qb.conditions, processedCondition)
	qb.args = append(qb.args, args...)
	return qb
}

// WhereEq adds an equality WHERE condition
func (qb *QueryBuilder) WhereEq(column string, value interface{}) *QueryBuilder {
	condition := fmt.Sprintf("%s = $%d", column, qb.argIndex)
	qb.conditions = append(qb.conditions, condition)
	qb.args = append(qb.args, value)
	qb.argIndex++
	return qb
}

// WhereIn adds an IN WHERE condition
func (qb *QueryBuilder) WhereIn(column string, values ...interface{}) *QueryBuilder {
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = fmt.Sprintf("$%d", qb.argIndex)
		qb.argIndex++
	}
	condition := fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ", "))
	qb.conditions = append(qb.conditions, condition)
	qb.args = append(qb.args, values...)
	return qb
}

// WhereNotNull adds a NOT NULL WHERE condition
func (qb *QueryBuilder) WhereNotNull(column string) *QueryBuilder {
	condition := fmt.Sprintf("%s IS NOT NULL", column)
	qb.conditions = append(qb.conditions, condition)
	return qb
}

// WhereNull adds a NULL WHERE condition
func (qb *QueryBuilder) WhereNull(column string) *QueryBuilder {
	condition := fmt.Sprintf("%s IS NULL", column)
	qb.conditions = append(qb.conditions, condition)
	return qb
}

// Join adds a JOIN clause
func (qb *QueryBuilder) Join(table, condition string) *QueryBuilder {
	join := fmt.Sprintf("JOIN %s ON %s", table, condition)
	qb.joins = append(qb.joins, join)
	return qb
}

// LeftJoin adds a LEFT JOIN clause
func (qb *QueryBuilder) LeftJoin(table, condition string) *QueryBuilder {
	join := fmt.Sprintf("LEFT JOIN %s ON %s", table, condition)
	qb.joins = append(qb.joins, join)
	return qb
}

// RightJoin adds a RIGHT JOIN clause
func (qb *QueryBuilder) RightJoin(table, condition string) *QueryBuilder {
	join := fmt.Sprintf("RIGHT JOIN %s ON %s", table, condition)
	qb.joins = append(qb.joins, join)
	return qb
}

// InnerJoin adds an INNER JOIN clause
func (qb *QueryBuilder) InnerJoin(table, condition string) *QueryBuilder {
	join := fmt.Sprintf("INNER JOIN %s ON %s", table, condition)
	qb.joins = append(qb.joins, join)
	return qb
}

// OrderBy adds an ORDER BY clause
func (qb *QueryBuilder) OrderBy(column string, direction ...string) *QueryBuilder {
	dir := "ASC"
	if len(direction) > 0 {
		dir = strings.ToUpper(direction[0])
	}
	order := fmt.Sprintf("%s %s", column, dir)
	qb.orderBy = append(qb.orderBy, order)
	return qb
}

// OrderByDesc adds an ORDER BY DESC clause
func (qb *QueryBuilder) OrderByDesc(column string) *QueryBuilder {
	return qb.OrderBy(column, "DESC")
}

// GroupBy adds a GROUP BY clause
func (qb *QueryBuilder) GroupBy(columns ...string) *QueryBuilder {
	qb.groupBy = append(qb.groupBy, columns...)
	return qb
}

// Having adds a HAVING clause
func (qb *QueryBuilder) Having(condition string, args ...interface{}) *QueryBuilder {
	processedCondition := qb.processPlaceholders(condition, len(args))
	qb.having = append(qb.having, processedCondition)
	qb.args = append(qb.args, args...)
	return qb
}

// Limit adds a LIMIT clause
func (qb *QueryBuilder) Limit(count int) *QueryBuilder {
	qb.limit = &count
	return qb
}

// Offset adds an OFFSET clause
func (qb *QueryBuilder) Offset(count int) *QueryBuilder {
	qb.offset = &count
	return qb
}

// OnConflict adds an ON CONFLICT clause for INSERT queries (PostgreSQL)
func (qb *QueryBuilder) OnConflict(columns ...string) *QueryBuilder {
	qb.conflicts = columns
	return qb
}

// DoNothing sets the conflict action to DO NOTHING
func (qb *QueryBuilder) DoNothing() *QueryBuilder {
	qb.conflictAction = "DO NOTHING"
	return qb
}

// DoUpdate sets the conflict action to DO UPDATE SET
func (qb *QueryBuilder) DoUpdate(updates map[string]interface{}) *QueryBuilder {
	setParts := make([]string, 0, len(updates))
	for column, value := range updates {
		placeholder := fmt.Sprintf("$%d", qb.argIndex)
		qb.argIndex++
		setParts = append(setParts, fmt.Sprintf("%s = %s", column, placeholder))
		qb.args = append(qb.args, value)
	}
	qb.conflictAction = "DO UPDATE SET " + strings.Join(setParts, ", ")
	return qb
}

// Build constructs the final SQL query and returns it with arguments
func (qb *QueryBuilder) Build() (string, []interface{}) {
	switch qb.queryType {
	case "SELECT":
		return qb.buildSelect(), qb.args
	case "INSERT":
		return qb.buildInsert(), qb.args
	case "UPDATE":
		return qb.buildUpdate(), qb.args
	case "DELETE":
		return qb.buildDelete(), qb.args
	default:
		return "", nil
	}
}

// buildSelect constructs a SELECT query
func (qb *QueryBuilder) buildSelect() string {
	var parts []string

	// SELECT clause
	if len(qb.columns) > 0 {
		parts = append(parts, "SELECT "+strings.Join(qb.columns, ", "))
	} else {
		parts = append(parts, "SELECT *")
	}

	// FROM clause
	if qb.table != "" {
		parts = append(parts, "FROM "+qb.table)
	}

	// JOIN clauses
	if len(qb.joins) > 0 {
		parts = append(parts, strings.Join(qb.joins, " "))
	}

	// WHERE clause
	if len(qb.conditions) > 0 {
		parts = append(parts, "WHERE "+strings.Join(qb.conditions, " AND "))
	}

	// GROUP BY clause
	if len(qb.groupBy) > 0 {
		parts = append(parts, "GROUP BY "+strings.Join(qb.groupBy, ", "))
	}

	// HAVING clause
	if len(qb.having) > 0 {
		parts = append(parts, "HAVING "+strings.Join(qb.having, " AND "))
	}

	// ORDER BY clause
	if len(qb.orderBy) > 0 {
		parts = append(parts, "ORDER BY "+strings.Join(qb.orderBy, ", "))
	}

	// LIMIT clause
	if qb.limit != nil {
		parts = append(parts, fmt.Sprintf("LIMIT %d", *qb.limit))
	}

	// OFFSET clause
	if qb.offset != nil {
		parts = append(parts, fmt.Sprintf("OFFSET %d", *qb.offset))
	}

	return strings.Join(parts, " ")
}

// buildInsert constructs an INSERT query
func (qb *QueryBuilder) buildInsert() string {
	var parts []string

	// INSERT INTO clause
	parts = append(parts, "INSERT INTO "+qb.table)

	// Columns clause
	if len(qb.columns) > 0 {
		parts = append(parts, "("+strings.Join(qb.columns, ", ")+")")
	}

	// VALUES clause
	if len(qb.placeholders) > 0 {
		parts = append(parts, "VALUES "+strings.Join(qb.placeholders, ", "))
	}

	// ON CONFLICT clause (PostgreSQL)
	if len(qb.conflicts) > 0 {
		conflictClause := "ON CONFLICT (" + strings.Join(qb.conflicts, ", ") + ")"
		if qb.conflictAction != "" {
			conflictClause += " " + qb.conflictAction
		}
		parts = append(parts, conflictClause)
	}

	return strings.Join(parts, " ")
}

// buildUpdate constructs an UPDATE query
func (qb *QueryBuilder) buildUpdate() string {
	var parts []string

	// UPDATE clause
	parts = append(parts, "UPDATE "+qb.table)

	// SET clause
	if len(qb.setConditions) > 0 {
		parts = append(parts, "SET "+strings.Join(qb.setConditions, ", "))
	}

	// WHERE clause
	if len(qb.conditions) > 0 {
		parts = append(parts, "WHERE "+strings.Join(qb.conditions, " AND "))
	}

	return strings.Join(parts, " ")
}

// buildDelete constructs a DELETE query
func (qb *QueryBuilder) buildDelete() string {
	var parts []string

	// DELETE FROM clause
	parts = append(parts, "DELETE FROM "+qb.table)

	// WHERE clause
	if len(qb.conditions) > 0 {
		parts = append(parts, "WHERE "+strings.Join(qb.conditions, " AND "))
	}

	return strings.Join(parts, " ")
}

// processPlaceholders converts ? placeholders to $n placeholders and updates argIndex
func (qb *QueryBuilder) processPlaceholders(condition string, argCount int) string {
	result := condition
	for i := 0; i < argCount; i++ {
		result = strings.Replace(result, "?", fmt.Sprintf("$%d", qb.argIndex), 1)
		qb.argIndex++
	}
	return result
}

// Reset resets the query builder to its initial state
func (qb *QueryBuilder) Reset() *QueryBuilder {
	qb.queryType = ""
	qb.table = ""
	qb.columns = qb.columns[:0]
	qb.values = qb.values[:0]
	qb.placeholders = qb.placeholders[:0]
	qb.conditions = qb.conditions[:0]
	qb.setConditions = qb.setConditions[:0]
	qb.joins = qb.joins[:0]
	qb.orderBy = qb.orderBy[:0]
	qb.groupBy = qb.groupBy[:0]
	qb.having = qb.having[:0]
	qb.limit = nil
	qb.offset = nil
	qb.args = qb.args[:0]
	qb.argIndex = 1
	qb.conflicts = qb.conflicts[:0]
	qb.conflictAction = ""
	return qb
}

// Clone creates a copy of the query builder
func (qb *QueryBuilder) Clone() *QueryBuilder {
	clone := &QueryBuilder{
		queryType:      qb.queryType,
		table:          qb.table,
		columns:        make([]string, len(qb.columns)),
		values:         make([]interface{}, len(qb.values)),
		placeholders:   make([]string, len(qb.placeholders)),
		conditions:     make([]string, len(qb.conditions)),
		setConditions:  make([]string, len(qb.setConditions)),
		joins:          make([]string, len(qb.joins)),
		orderBy:        make([]string, len(qb.orderBy)),
		groupBy:        make([]string, len(qb.groupBy)),
		having:         make([]string, len(qb.having)),
		args:           make([]interface{}, len(qb.args)),
		argIndex:       qb.argIndex,
		conflicts:      make([]string, len(qb.conflicts)),
		conflictAction: qb.conflictAction,
	}

	copy(clone.columns, qb.columns)
	copy(clone.values, qb.values)
	copy(clone.placeholders, qb.placeholders)
	copy(clone.conditions, qb.conditions)
	copy(clone.setConditions, qb.setConditions)
	copy(clone.joins, qb.joins)
	copy(clone.orderBy, qb.orderBy)
	copy(clone.groupBy, qb.groupBy)
	copy(clone.having, qb.having)
	copy(clone.args, qb.args)
	copy(clone.conflicts, qb.conflicts)

	if qb.limit != nil {
		limitCopy := *qb.limit
		clone.limit = &limitCopy
	}

	if qb.offset != nil {
		offsetCopy := *qb.offset
		clone.offset = &offsetCopy
	}

	return clone
}
