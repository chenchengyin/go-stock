package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"go-stock/backend/db"
	"go-stock/backend/flutter_api"

	"golang.org/x/term"
)

func main() {
	account := flag.String("account", "", "administrator account")
	nickname := flag.String("nickname", "", "administrator nickname")
	flag.Parse()
	if strings.TrimSpace(*account) == "" {
		log.Fatal("-account is required")
	}

	password := readPassword("Password: ")
	confirmation := readPassword("Confirm password: ")
	if password != confirmation {
		log.Fatal("passwords do not match")
	}

	db.Init("")
	if err := flutter_api.MigrateAuthTables(db.Dao); err != nil {
		log.Fatal(err)
	}
	if err := flutter_api.CreateAdmin(context.Background(), db.Dao, flutter_api.AdminInitInput{
		Account: *account, Nickname: *nickname, Password: password,
	}); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintln(os.Stdout, "administrator created")
}

func readPassword(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		log.Fatal(err)
	}
	return string(password)
}
