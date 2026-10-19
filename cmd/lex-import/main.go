// Command lex-import loads the official texts the AI tutor answers from into
// the legal_paragraphs table, from lex.uz:
//
//	yhq   Yo'l harakati qoidalari (VM 172-son qarori)
//	law   "Yo'l harakati to'g'risida"gi Qonun (O'RQ-900)
//	mjtk  Ma'muriy javobgarlik to'g'risidagi kodeks, XI bob (traffic fines)
//
//	go run ./cmd/lex-import                       # fetch all three and import
//	go run ./cmd/lex-import -doc mjtk             # one document
//	go run ./cmd/lex-import -save data/lex        # also keep the fetched pages
//	go run ./cmd/lex-import -from data/lex        # import saved pages (lex-<id>.html)
//	go run ./cmd/lex-import -dry-run              # parse and summarise only
//
// Each document's rows are replaced as a whole, so re-running after lex.uz
// publishes an amendment brings the tutor up to date. The running API picks
// the new text up on its next corpus refresh (at most 15 minutes).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/avtofast/avtofast-back/internal/config"
	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/lexuz"
	"github.com/avtofast/avtofast-back/internal/repository/postgres"
)

func fetch(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// lex.uz serves an empty shell to clients it does not take for a browser.
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) avtofast-lex-import")
	client := &http.Client{Timeout: 3 * time.Minute}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, res.Status)
	}
	return io.ReadAll(res.Body)
}

// pageFile is where a document's page is saved: lex-<id>.html.
func pageFile(dir string, d lexuz.Document) string {
	return filepath.Join(dir, "lex-"+strings.TrimPrefix(d.URL[strings.LastIndex(d.URL, "/")+1:], "-")+".html")
}

func main() {
	only := flag.String("doc", "", "import one document: yhq, law or mjtk (default: all)")
	from := flag.String("from", "", "read saved pages from this directory instead of fetching")
	save := flag.String("save", "", "write fetched pages to this directory")
	dryRun := flag.Bool("dry-run", false, "parse and print a summary without writing to the database")
	flag.Parse()

	docs := lexuz.Documents
	if *only != "" {
		d, ok := lexuz.DocumentByCode(*only)
		if !ok {
			fmt.Fprintf(os.Stderr, "lex-import: unknown document %q\n", *only)
			os.Exit(2)
		}
		docs = []lexuz.Document{d}
	}

	parsed := map[string][]domain.LegalParagraph{}
	for _, d := range docs {
		var page []byte
		var err error
		if *from != "" {
			page, err = os.ReadFile(pageFile(*from, d))
		} else {
			page, err = fetch(d.URL)
			if err == nil && *save != "" {
				err = os.WriteFile(pageFile(*save, d), page, 0o644)
			}
		}
		if err == nil {
			parsed[d.Code], err = d.Parse(page)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "lex-import: %s: %v\n", d.Code, err)
			os.Exit(1)
		}

		chapters, clauses := map[string]bool{}, map[string]bool{}
		for _, p := range parsed[d.Code] {
			chapters[p.Chapter+p.Appendix] = true
			clauses[p.Chapter+p.Appendix+"|"+p.ClauseNumber] = true
		}
		fmt.Printf("%-5s %5d paragraphs, %3d chapters/appendices, %4d clauses/articles  (%s)\n",
			d.Code, len(parsed[d.Code]), len(chapters), len(clauses), d.Title)
	}
	if *dryRun {
		return
	}

	cfg, _ := config.Load("config/config.yaml")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lex-import: database:", err)
		os.Exit(1)
	}
	defer db.Close()

	repo := postgres.NewLegalRepository(db)
	for _, d := range docs {
		if err := repo.ReplaceDocument(ctx, d.Code, parsed[d.Code]); err != nil {
			fmt.Fprintf(os.Stderr, "lex-import: %s: write: %v\n", d.Code, err)
			os.Exit(1)
		}
		fmt.Printf("imported %s\n", d.Code)
	}
}
