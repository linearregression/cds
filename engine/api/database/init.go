package database

import (
	"database/sql"
	"os"
	"regexp"

	"github.com/rubenv/sql-migrate/sqlparse"
)

// InitSchemas checks that all tables are correct, and create them if not
func InitSchemas(sqlDB *sql.DB, sqlfile string) error {
	f, err := os.Open(sqlfile)
	if err != nil {
		return err
	}
	defer f.Close()

	queries, err := sqlparse.SplitSQLStatements(f, true)
	if err != nil {
		return err
	}

	//Do not consider comments
	r, _ := regexp.Compile("--.*\n")

	for _, q := range queries {
		s := r.ReplaceAllString(q, "")
		_, err := sqlDB.Exec(s)
		if err != nil {
			return err
		}
	}
	return nil
}
