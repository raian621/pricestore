package db

import (
	"context"
	"log"
	"os"
	"path"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Migration struct {
	Name   string
	Script string
}

func (m Migration) Apply(p *pgxpool.Pool) error {
	row := p.QueryRow(context.Background(),
		"SELECT COUNT(*) > 0 FROM migrations WHERE migration_name = $1", m.Name)
	var migrationApplied bool
	if err := row.Scan(&migrationApplied); err != nil {
		return err
	}
	if migrationApplied {
		log.Printf("Skipping migration %s", m.Name)
		return nil
	}
	log.Printf("Executing migration %s", m.Name)
	_, err := p.Exec(context.Background(), m.Script)
	return err
}

func ReadMigrations(baseDir string) ([]Migration, error) {
	entries, err := os.ReadDir(baseDir)
	if err == os.ErrNotExist || err != nil {
		log.Printf("`baseDir` migrations directory does not exist: %s", baseDir)
		return nil, err
	}
	migrations := make([]Migration, 0)

	for _, entry := range entries {
		fileName := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(fileName, ".sql") {
			log.Println("Skipping file:", fileName)
			continue
		}
		filePath := path.Join(baseDir, fileName)
		if scriptBytes, err := os.ReadFile(filePath); err != nil {
			return nil, err
		} else {
			migrations = append(migrations, Migration{
				Name: fileName, Script: string(scriptBytes)})
		}
	}

	return migrations, nil
}

func ApplyMigrations(p *pgxpool.Pool, baseDir string, bootstrap bool) error {
	if bootstrap {
		BootstrapMigrationTable(p)
	}
	migrations, err := ReadMigrations(baseDir)
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		if err := migration.Apply(p); err != nil {
			return err
		}
	}
	return nil
}

func BootstrapMigrationTable(p *pgxpool.Pool) {
	log.Println("Bootsrapping migrations table")
	_, err := p.Exec(context.Background(),
		"CREATE TABLE migrations (migration_name VARCHAR(255) UNIQUE)")
	if err != nil {
		log.Fatalln(err)
	}
}
