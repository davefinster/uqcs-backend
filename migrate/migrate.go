package main

import (
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	m, err := migrate.New(
		"file://./migrate",
		os.Getenv("CONNECTION_STRING"),
	)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}
	err = m.Migrate(20200516153823)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}
}
