// Command corpusgen writes a deterministic, seeded "messy" knowledge base for
// eval and manual dogfooding. Development tooling — not part of the dig product.
//
//	go run ./tools/corpusgen --seed 42 --size medium --out /tmp/messy-kb
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/vllnt/dig/internal/corpus"
)

func main() {
	seed := flag.Int64("seed", 1, "RNG seed — same seed yields a byte-identical tree")
	size := flag.String("size", "medium", "corpus size: small | medium | large")
	out := flag.String("out", "", "output directory (required)")
	asJSON := flag.Bool("json", false, "print the corpus spec as JSON")
	flag.Parse()

	if *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	spec, err := corpus.Generate(*out, *seed, corpus.Size(*size))
	if err != nil {
		log.Fatal(err)
	}
	if *asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(spec)
		return
	}
	log.Printf("wrote %d files (%d docs, %d dupes, %d binaries) to %s",
		spec.Files, spec.Documents, spec.Duplicates, spec.Binaries, *out)
}
