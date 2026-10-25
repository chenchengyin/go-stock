package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"go-stock/backend/flutter_api"
)

func main() {
	cache := flag.String("cache", "backend/data/cache", "cache root")
	recoverPools := flag.Bool("recover-pools", false, "archive only existing witnessed, complete dated stock pools")
	output := flag.String("output", "", "JSON report path; default stdout")
	requireComplete := flag.Bool("require-complete", false, "exit 2 when history is incomplete")
	flag.Parse()
	report, err := flutter_api.AuditT0History(*cache, *recoverPools)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out := os.Stdout
	if *output != "" {
		if err = os.MkdirAll(filepath.Dir(*output), 0755); err == nil {
			out, err = os.Create(*output)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer out.Close()
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err = enc.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *requireComplete && (report.MissingPoolDates > 0 || len(report.Errors) > 0) {
		os.Exit(2)
	}
}
