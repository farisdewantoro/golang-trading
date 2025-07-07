package cmd

import (
	"fmt"
	"log"

	"golang-trading/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/cobra"
)

var configPath string

func getDSN(dbConfig config.Database) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		dbConfig.User,
		dbConfig.Password,
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.DBName,
		dbConfig.SSLMode)
}

func runMigrations(direction string) {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	dsn := getDSN(cfg.DB)
	migrationsPath := "file://migrations"

	m, err := migrate.New(migrationsPath, dsn)
	if err != nil {
		log.Fatalf("Failed to create migration instance: %v", err)
	}

	var migrationErr error
	if direction == "up" {
		migrationErr = m.Up()
		fmt.Println("Applied migrations successfully.")
	} else if direction == "down" {
		migrationErr = m.Steps(-1)
		fmt.Println("Reverted last migration successfully.")
	}

	if migrationErr != nil && migrationErr != migrate.ErrNoChange {
		log.Fatalf("Migration failed: %v", migrationErr)
	}

	srcErr, dbErr := m.Close()
	if srcErr != nil {
		log.Printf("Migration source error on close: %v\n", srcErr)
	}
	if dbErr != nil {
		log.Printf("Migration database error on close: %v\n", dbErr)
	}
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply all available database migrations",
	Run: func(cmd *cobra.Command, args []string) {
		runMigrations("up")
	},
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Revert the last database migration",
	Run: func(cmd *cobra.Command, args []string) {
		runMigrations("down")
	},
}

var migrateCmd = &cobra.Command{
	Use: "migrate",
}

func init() {
	migrateCmd.AddCommand(upCmd)
	migrateCmd.AddCommand(downCmd)
}
