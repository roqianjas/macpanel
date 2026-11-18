// Command genapi menulis (atau memeriksa) docs/openapi.json dan ui/src/api.gen.ts dari tabel rute Go.
//
//	go run ./cmd/genapi          tulis berkas
//	go run ./cmd/genapi -check   gagal (exit 1) bila berkas di disk berbeda dari yang seharusnya
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/roqianjas/macpanel/internal/apispec"
	"github.com/roqianjas/macpanel/internal/server"
)

func main() {
	check := flag.Bool("check", false, "hanya periksa, jangan menulis")
	flag.Parse()
	ops, err := apispec.Ops(server.Routes())
	if err != nil {
		fmt.Fprintln(os.Stderr, "genapi:", err)
		os.Exit(2)
	}
	spec, err := apispec.OpenAPI(ops)
	if err != nil {
		fmt.Fprintln(os.Stderr, "genapi:", err)
		os.Exit(2)
	}
	files := map[string][]byte{"docs/openapi.json": spec, "ui/src/api.gen.ts": apispec.TypeScript(ops)}
	bad := false
	for _, name := range []string{"docs/openapi.json", "ui/src/api.gen.ts"} {
		want := files[name]
		if *check {
			got, err := os.ReadFile(name)
			if err != nil || !bytes.Equal(got, want) {
				fmt.Fprintf(os.Stderr, "genapi: %s tidak sinkron dengan rute Go. Jalankan: make gen-api\n", name)
				bad = true
			}
			continue
		}
		if err := os.WriteFile(name, want, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "genapi:", err)
			os.Exit(2)
		}
		fmt.Printf("ditulis: %s (%d operasi)\n", name, len(ops))
	}
	if bad {
		os.Exit(1)
	}
}
