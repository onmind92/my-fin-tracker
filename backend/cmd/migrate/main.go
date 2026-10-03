package main

import (
	"errors"
	"fmt"
	"os"

	"finance-tracker/backend/internal/config"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	action := "up"
	if len(os.Args) > 1 {
		action = os.Args[1]
	}
	m, err := migrate.New("file://migrations", config.Load().DatabaseURL)
	if err != nil {
		fatal(err)
	}
	defer m.Close()
	switch action {
	case "up":
		err = m.Up()
	case "down":
		err = m.Steps(-1)
	default:
		fatal(fmt.Errorf("usage: migrate [up|down]"))
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		fatal(err)
	}
	fmt.Println("migrations:", action, "complete")
}
func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
