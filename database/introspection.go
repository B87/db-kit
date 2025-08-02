package database

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// IntrospectionService provides database schema introspection capabilities
type IntrospectionService struct {
	db *DB
}

// NewIntrospectionService creates a new introspection service
func NewIntrospectionService(db *DB) *IntrospectionService {
	return &IntrospectionService{db: db}
}

// TableInfo represents information about a database table
type TableInfo struct {
	Name        string           `json:"name" db:"table_name"`
	Schema      string           `json:"schema" db:"table_schema"`
	Type        string           `json:"type" db:"table_type"`
	Comment     *string          `json:"comment,omitempty" db:"table_comment"`
	Columns     []ColumnInfo     `json:"columns,omitempty"`
	Indexes     []IndexInfo      `json:"indexes,omitempty"`
	Constraints []ConstraintInfo `json:"constraints,omitempty"`
}

// ColumnInfo represents information about a table column
type ColumnInfo struct {
	Name             string  `json:"name" db:"column_name"`
	DataType         string  `json:"data_type" db:"data_type"`
	IsNullable       bool    `json:"is_nullable" db:"is_nullable"`
	DefaultValue     *string `json:"default_value,omitempty" db:"column_default"`
	IsPrimaryKey     bool    `json:"is_primary_key" db:"is_primary_key"`
	IsForeignKey     bool    `json:"is_foreign_key" db:"is_foreign_key"`
	IsUnique         bool    `json:"is_unique" db:"is_unique"`
	MaxLength        *int    `json:"max_length,omitempty" db:"character_maximum_length"`
	NumericPrecision *int    `json:"numeric_precision,omitempty" db:"numeric_precision"`
	NumericScale     *int    `json:"numeric_scale,omitempty" db:"numeric_scale"`
	Comment          *string `json:"comment,omitempty" db:"column_comment"`
}

// IndexInfo represents information about a table index
type IndexInfo struct {
	Name      string   `json:"name" db:"index_name"`
	TableName string   `json:"table_name" db:"table_name"`
	Columns   []string `json:"columns"`
	IsUnique  bool     `json:"is_unique" db:"is_unique"`
	IsPrimary bool     `json:"is_primary" db:"is_primary"`
	IndexType string   `json:"index_type" db:"index_type"`
}

// ConstraintInfo represents information about table constraints
type ConstraintInfo struct {
	Name              string   `json:"name" db:"constraint_name"`
	Type              string   `json:"type" db:"constraint_type"`
	TableName         string   `json:"table_name" db:"table_name"`
	Columns           []string `json:"columns"`
	ReferencedTable   *string  `json:"referenced_table,omitempty" db:"referenced_table_name"`
	ReferencedColumns []string `json:"referenced_columns,omitempty"`
	UpdateRule        *string  `json:"update_rule,omitempty" db:"update_rule"`
	DeleteRule        *string  `json:"delete_rule,omitempty" db:"delete_rule"`
}

// DatabaseInfo represents overall database information
type DatabaseInfo struct {
	Name    string      `json:"name"`
	Version string      `json:"version"`
	Size    *int64      `json:"size,omitempty"`
	Tables  []TableInfo `json:"tables,omitempty"`
	Schemas []string    `json:"schemas,omitempty"`
}

// GetDatabaseInfo retrieves comprehensive database information
func (is *IntrospectionService) GetDatabaseInfo(ctx context.Context) (*DatabaseInfo, error) {
	info := &DatabaseInfo{
		Name: is.db.config.DBName,
	}

	// Get database version
	version, err := is.GetDatabaseVersion(ctx)
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "get_database_info", "failed to get database version")
	}
	info.Version = version

	// Get database size
	size, err := is.GetDatabaseSize(ctx)
	if err != nil {
		// Size is optional, log but don't fail
		is.db.logger.Warn("failed to get database size", "error", err)
	} else {
		info.Size = &size
	}

	// Get schemas
	schemas, err := is.GetSchemas(ctx)
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "get_database_info", "failed to get schemas")
	}
	info.Schemas = schemas

	// Get tables
	tables, err := is.GetTables(ctx, "")
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "get_database_info", "failed to get tables")
	}
	info.Tables = tables

	return info, nil
}

// GetDatabaseVersion retrieves the PostgreSQL version
func (is *IntrospectionService) GetDatabaseVersion(ctx context.Context) (string, error) {
	var version string
	err := is.db.WithValidation(ctx, func() error {
		return is.db.db.GetContext(ctx, &version, "SELECT version()")
	})
	if err != nil {
		return "", WrapError(err, ErrCodeQueryFailed, "get_database_version", "failed to get database version")
	}
	return version, nil
}

// GetDatabaseSize retrieves the size of the database in bytes
func (is *IntrospectionService) GetDatabaseSize(ctx context.Context) (int64, error) {
	var size int64
	err := is.db.WithValidation(ctx, func() error {
		return is.db.db.GetContext(ctx, &size,
			"SELECT pg_database_size($1)", is.db.config.DBName)
	})
	if err != nil {
		return 0, WrapError(err, ErrCodeQueryFailed, "get_database_size", "failed to get database size")
	}
	return size, nil
}

// GetSchemas retrieves all schemas in the database
func (is *IntrospectionService) GetSchemas(ctx context.Context) ([]string, error) {
	var schemas []string
	err := is.db.WithValidation(ctx, func() error {
		return is.db.db.SelectContext(ctx, &schemas, `
			SELECT schema_name
			FROM information_schema.schemata
			WHERE schema_name NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
			ORDER BY schema_name
		`)
	})
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "get_schemas", "failed to get schemas")
	}
	return schemas, nil
}

// GetTables retrieves all tables in the specified schema (empty string for all schemas)
func (is *IntrospectionService) GetTables(ctx context.Context, schema string) ([]TableInfo, error) {
	var tables []TableInfo

	query := `
		SELECT
			t.table_name,
			t.table_schema,
			t.table_type,
			obj_description(c.oid) as table_comment
		FROM information_schema.tables t
		LEFT JOIN pg_class c ON c.relname = t.table_name
		LEFT JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = t.table_schema
		WHERE t.table_schema NOT IN ('information_schema', 'pg_catalog')
	`

	args := []interface{}{}
	if schema != "" {
		query += " AND t.table_schema = $1"
		args = append(args, schema)
	}

	query += " ORDER BY t.table_schema, t.table_name"

	err := is.db.WithValidation(ctx, func() error {
		return is.db.db.SelectContext(ctx, &tables, query, args...)
	})
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "get_tables", "failed to get tables")
	}

	// Get detailed information for each table
	for i := range tables {
		// Get columns
		columns, err := is.GetTableColumns(ctx, tables[i].Schema, tables[i].Name)
		if err != nil {
			return nil, WrapError(err, ErrCodeQueryFailed, "get_tables", fmt.Sprintf("failed to get columns for table %s.%s", tables[i].Schema, tables[i].Name))
		}
		tables[i].Columns = columns

		// Get indexes
		indexes, err := is.GetTableIndexes(ctx, tables[i].Schema, tables[i].Name)
		if err != nil {
			return nil, WrapError(err, ErrCodeQueryFailed, "get_tables", fmt.Sprintf("failed to get indexes for table %s.%s", tables[i].Schema, tables[i].Name))
		}
		tables[i].Indexes = indexes

		// Get constraints - make this optional to avoid timeouts
		constraints, err := is.GetTableConstraints(ctx, tables[i].Schema, tables[i].Name)
		if err != nil {
			// Log warning but don't fail the entire operation
			is.db.logger.Warn("failed to get constraints for table",
				"schema", tables[i].Schema,
				"table", tables[i].Name,
				"error", err)
			// Set empty constraints instead of failing
			tables[i].Constraints = []ConstraintInfo{}
		} else {
			tables[i].Constraints = constraints
		}
	}

	return tables, nil
}

// GetTableColumns retrieves columns for a specific table
func (is *IntrospectionService) GetTableColumns(ctx context.Context, schema, tableName string) ([]ColumnInfo, error) {
	var columns []ColumnInfo

	query := `
		SELECT
			c.column_name,
			c.data_type,
			CASE WHEN c.is_nullable = 'YES' THEN true ELSE false END as is_nullable,
			c.column_default,
			c.character_maximum_length,
			c.numeric_precision,
			c.numeric_scale,
			CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END as is_primary_key,
			CASE WHEN fk.column_name IS NOT NULL THEN true ELSE false END as is_foreign_key,
			CASE WHEN uk.column_name IS NOT NULL THEN true ELSE false END as is_unique,
			col_description(pgc.oid, c.ordinal_position) as column_comment
		FROM information_schema.columns c
		LEFT JOIN pg_class pgc ON pgc.relname = c.table_name
		LEFT JOIN pg_namespace pgn ON pgn.oid = pgc.relnamespace AND pgn.nspname = c.table_schema
		LEFT JOIN (
			SELECT ku.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage ku ON tc.constraint_name = ku.constraint_name
			WHERE tc.constraint_type = 'PRIMARY KEY'
			AND tc.table_schema = $1 AND tc.table_name = $2
		) pk ON pk.column_name = c.column_name
		LEFT JOIN (
			SELECT ku.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage ku ON tc.constraint_name = ku.constraint_name
			WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = $1 AND tc.table_name = $2
		) fk ON fk.column_name = c.column_name
		LEFT JOIN (
			SELECT ku.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage ku ON tc.constraint_name = ku.constraint_name
			WHERE tc.constraint_type = 'UNIQUE'
			AND tc.table_schema = $1 AND tc.table_name = $2
		) uk ON uk.column_name = c.column_name
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position
	`

	err := is.db.WithValidation(ctx, func() error {
		return is.db.db.SelectContext(ctx, &columns, query, schema, tableName)
	})
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "get_table_columns", "failed to get table columns")
	}

	return columns, nil
}

// GetTableIndexes retrieves indexes for a specific table
func (is *IntrospectionService) GetTableIndexes(ctx context.Context, schema, tableName string) ([]IndexInfo, error) {
	type indexRow struct {
		IndexName  string `db:"index_name"`
		TableName  string `db:"table_name"`
		ColumnName string `db:"column_name"`
		IsUnique   bool   `db:"is_unique"`
		IsPrimary  bool   `db:"is_primary"`
		IndexType  string `db:"index_type"`
	}

	var rows []indexRow
	query := `
		SELECT
			i.relname as index_name,
			t.relname as table_name,
			a.attname as column_name,
			ix.indisunique as is_unique,
			ix.indisprimary as is_primary,
			am.amname as index_type
		FROM pg_class t
		JOIN pg_namespace n ON n.oid = t.relnamespace
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_am am ON i.relam = am.oid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE n.nspname = $1 AND t.relname = $2
		ORDER BY i.relname, a.attnum
	`

	err := is.db.WithValidation(ctx, func() error {
		return is.db.db.SelectContext(ctx, &rows, query, schema, tableName)
	})
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "get_table_indexes", "failed to get table indexes")
	}

	// Group by index name
	indexMap := make(map[string]*IndexInfo)
	for _, row := range rows {
		if idx, exists := indexMap[row.IndexName]; exists {
			idx.Columns = append(idx.Columns, row.ColumnName)
		} else {
			indexMap[row.IndexName] = &IndexInfo{
				Name:      row.IndexName,
				TableName: row.TableName,
				Columns:   []string{row.ColumnName},
				IsUnique:  row.IsUnique,
				IsPrimary: row.IsPrimary,
				IndexType: row.IndexType,
			}
		}
	}

	// Convert map to slice
	var indexes []IndexInfo
	for _, idx := range indexMap {
		indexes = append(indexes, *idx)
	}

	return indexes, nil
}

// GetTableConstraints retrieves constraints for a specific table
func (is *IntrospectionService) GetTableConstraints(ctx context.Context, schema, tableName string) ([]ConstraintInfo, error) {
	type constraintRow struct {
		ConstraintName       string  `db:"constraint_name"`
		ConstraintType       string  `db:"constraint_type"`
		TableName            string  `db:"table_name"`
		ColumnName           string  `db:"column_name"`
		ReferencedTableName  *string `db:"referenced_table_name"`
		ReferencedColumnName *string `db:"referenced_column_name"`
		UpdateRule           *string `db:"update_rule"`
		DeleteRule           *string `db:"delete_rule"`
	}

	var rows []constraintRow
	query := `
		SELECT
			tc.constraint_name,
			tc.constraint_type,
			tc.table_name,
			kcu.column_name,
			ccu.table_name as referenced_table_name,
			ccu.column_name as referenced_column_name,
			rc.update_rule,
			rc.delete_rule
		FROM information_schema.table_constraints tc
		LEFT JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		LEFT JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name
			AND tc.table_schema = ccu.table_schema
		LEFT JOIN information_schema.referential_constraints rc
			ON tc.constraint_name = rc.constraint_name
			AND tc.table_schema = rc.constraint_schema
		WHERE tc.table_schema = $1 AND tc.table_name = $2
		ORDER BY tc.constraint_name, kcu.ordinal_position
	`

	// Use a shorter timeout for constraint queries to prevent hanging
	constraintCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := is.db.WithValidation(constraintCtx, func() error {
		return is.db.db.SelectContext(constraintCtx, &rows, query, schema, tableName)
	})
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "get_table_constraints", "failed to get table constraints")
	}

	// Group by constraint name
	constraintMap := make(map[string]*ConstraintInfo)
	for _, row := range rows {
		if constraint, exists := constraintMap[row.ConstraintName]; exists {
			if row.ColumnName != "" {
				constraint.Columns = append(constraint.Columns, row.ColumnName)
			}
			if row.ReferencedColumnName != nil {
				constraint.ReferencedColumns = append(constraint.ReferencedColumns, *row.ReferencedColumnName)
			}
		} else {
			columns := []string{}
			if row.ColumnName != "" {
				columns = append(columns, row.ColumnName)
			}

			referencedColumns := []string{}
			if row.ReferencedColumnName != nil {
				referencedColumns = append(referencedColumns, *row.ReferencedColumnName)
			}

			constraintMap[row.ConstraintName] = &ConstraintInfo{
				Name:              row.ConstraintName,
				Type:              row.ConstraintType,
				TableName:         row.TableName,
				Columns:           columns,
				ReferencedTable:   row.ReferencedTableName,
				ReferencedColumns: referencedColumns,
				UpdateRule:        row.UpdateRule,
				DeleteRule:        row.DeleteRule,
			}
		}
	}

	// Convert map to slice
	var constraints []ConstraintInfo
	for _, constraint := range constraintMap {
		constraints = append(constraints, *constraint)
	}

	return constraints, nil
}

// GetTableExists checks if a table exists in the database
func (is *IntrospectionService) GetTableExists(ctx context.Context, schema, tableName string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = $1 AND table_name = $2
		)
	`

	err := is.db.WithValidation(ctx, func() error {
		return is.db.db.GetContext(ctx, &exists, query, schema, tableName)
	})
	if err != nil {
		return false, WrapError(err, ErrCodeQueryFailed, "get_table_exists", "failed to check table existence")
	}

	return exists, nil
}

// GetColumnExists checks if a column exists in a specific table
func (is *IntrospectionService) GetColumnExists(ctx context.Context, schema, tableName, columnName string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = $2 AND column_name = $3
		)
	`

	err := is.db.WithValidation(ctx, func() error {
		return is.db.db.GetContext(ctx, &exists, query, schema, tableName, columnName)
	})
	if err != nil {
		return false, WrapError(err, ErrCodeQueryFailed, "get_column_exists", "failed to check column existence")
	}

	return exists, nil
}

// GetForeignKeyRelationships retrieves all foreign key relationships in the database
func (is *IntrospectionService) GetForeignKeyRelationships(ctx context.Context, schema string) ([]ConstraintInfo, error) {
	var constraints []ConstraintInfo

	query := `
		SELECT
			tc.constraint_name,
			tc.constraint_type,
			tc.table_name,
			ARRAY_AGG(kcu.column_name ORDER BY kcu.ordinal_position) as columns,
			ccu.table_name as referenced_table_name,
			ARRAY_AGG(ccu.column_name ORDER BY kcu.ordinal_position) as referenced_columns,
			rc.update_rule,
			rc.delete_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name
			AND tc.table_schema = ccu.table_schema
		JOIN information_schema.referential_constraints rc
			ON tc.constraint_name = rc.constraint_name
			AND tc.table_schema = rc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
	`

	args := []interface{}{}
	if schema != "" {
		query += " AND tc.table_schema = $1"
		args = append(args, schema)
	}

	query += `
		GROUP BY tc.constraint_name, tc.constraint_type, tc.table_name,
				 ccu.table_name, rc.update_rule, rc.delete_rule
		ORDER BY tc.table_name, tc.constraint_name
	`

	type fkRow struct {
		ConstraintName      string  `db:"constraint_name"`
		ConstraintType      string  `db:"constraint_type"`
		TableName           string  `db:"table_name"`
		Columns             string  `db:"columns"` // PostgreSQL array as string
		ReferencedTableName string  `db:"referenced_table_name"`
		ReferencedColumns   string  `db:"referenced_columns"` // PostgreSQL array as string
		UpdateRule          *string `db:"update_rule"`
		DeleteRule          *string `db:"delete_rule"`
	}

	var rows []fkRow
	err := is.db.WithValidation(ctx, func() error {
		return is.db.db.SelectContext(ctx, &rows, query, args...)
	})
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "get_foreign_key_relationships", "failed to get foreign key relationships")
	}

	// Convert rows to constraints
	for _, row := range rows {
		// Parse PostgreSQL array format: {col1,col2,col3}
		columns := parsePostgreSQLArray(row.Columns)
		referencedColumns := parsePostgreSQLArray(row.ReferencedColumns)

		constraint := ConstraintInfo{
			Name:              row.ConstraintName,
			Type:              row.ConstraintType,
			TableName:         row.TableName,
			Columns:           columns,
			ReferencedTable:   &row.ReferencedTableName,
			ReferencedColumns: referencedColumns,
			UpdateRule:        row.UpdateRule,
			DeleteRule:        row.DeleteRule,
		}
		constraints = append(constraints, constraint)
	}

	return constraints, nil
}

// parsePostgreSQLArray parses PostgreSQL array format {item1,item2,item3} into Go slice
func parsePostgreSQLArray(arrayStr string) []string {
	if arrayStr == "" || arrayStr == "{}" {
		return []string{}
	}

	// Remove braces
	arrayStr = strings.Trim(arrayStr, "{}")

	// Split by comma
	items := strings.Split(arrayStr, ",")

	// Trim whitespace from each item
	for i, item := range items {
		items[i] = strings.TrimSpace(item)
	}

	return items
}
