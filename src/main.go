package main

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raian621/pricestore/src/db"
)

func main() {
	var config Config
	if err := LoadConfigFromEnv(&config); err != nil {
		log.Fatalln(err)
	}

	pool, err := pgxpool.New(context.Background(), db.CreateDbConnString(config.DbUser, config.DbPassword, config.DbHost, config.DbPort, config.DbName))
	if err != nil {
		log.Fatalln(err)
	}
	if err := db.ApplyMigrations(pool, config.DbMigrationPath, config.BootstrapDb); err != nil {
		log.Fatalln(err)
	}
}
