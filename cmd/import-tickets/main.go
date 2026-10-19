// Command import-tickets stores Prepdrive's numbered test templates as exam
// tickets ("biletlar"): ticket N holds template N's questions in its order.
//
// It reads the raw template JSON cmd/scraper keeps (-raw-dir), so it needs no
// network and no Prepdrive token. Run it after the scraper, once the
// questions themselves are in the bank; it is safe to run again.
//
//	go run ./cmd/import-tickets
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/avtofast/avtofast-back/internal/config"
	"github.com/avtofast/avtofast-back/internal/repository/postgres"
)

type template struct {
	Success bool `json:"success"`
	Data    struct {
		Template struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"template"`
		Questions []struct {
			ID int64 `json:"id"`
		} `json:"questions"`
	} `json:"data"`
}

func main() {
	rawDir := flag.String("raw-dir", "./data/prepdrive/uz/test-templates", "directory of <n>.json template responses")
	packID := flag.String("pack-id", "uz-theory-2026-09", "question pack the tickets belong to")
	dryRun := flag.Bool("dry-run", false, "print what would be stored without saving")
	flag.Parse()

	tickets, err := readTemplates(*rawDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if len(tickets) == 0 {
		fmt.Fprintln(os.Stderr, "no templates found in", *rawDir, "— run cmd/scraper first")
		os.Exit(1)
	}

	slots, unique := 0, map[string]bool{}
	for _, t := range tickets {
		slots += len(t.QuestionIDs)
		for _, id := range t.QuestionIDs {
			unique[id] = true
		}
	}
	fmt.Printf("%d tickets, %d question slots, %d distinct questions\n", len(tickets), slots, len(unique))
	if *dryRun {
		return
	}

	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	ctx := context.Background()
	db, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "database:", err)
		os.Exit(1)
	}
	defer db.Close()

	repo := postgres.NewTicketRepository(db, postgres.NewContentRepository(db))

	// Prepdrive occasionally holds one question under two ids. Keep one, and
	// let every ticket point at it.
	dups, err := repo.MarkContentDuplicates(ctx, "uz-Latn-UZ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "duplicates:", err)
		os.Exit(1)
	}
	for dup, kept := range dups {
		fmt.Printf("duplicate question %s is the same as %s; keeping %s\n", dup, kept, kept)
	}
	for i := range tickets {
		for j, id := range tickets[i].QuestionIDs {
			if kept, ok := dups[id]; ok {
				tickets[i].QuestionIDs[j] = kept
			}
		}
	}

	missing, err := repo.ReplaceTickets(ctx, *packID, tickets)
	if err != nil {
		fmt.Fprintln(os.Stderr, "save:", err)
		os.Exit(1)
	}
	fmt.Printf("saved %d tickets\n", len(tickets))
	if len(missing) > 0 {
		fmt.Printf("%d questions are not in the bank and were left out: %s\n", len(missing), strings.Join(missing, ", "))
		fmt.Println("import them with cmd/scraper, then run this again")
	}
}

// readTemplates turns every <n>.json in dir into a ticket numbered n.
func readTemplates(dir string) ([]postgres.TicketImport, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []postgres.TicketImport
	for _, e := range entries {
		number, err := strconv.Atoi(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil || e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var t template
		if err := json.Unmarshal(b, &t); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if !t.Success || len(t.Data.Questions) == 0 {
			continue
		}
		ids := make([]string, 0, len(t.Data.Questions))
		for _, q := range t.Data.Questions {
			// The same ids cmd/scraper gives the questions it imports.
			ids = append(ids, fmt.Sprintf("pd_%d", q.ID))
		}
		out = append(out, postgres.TicketImport{
			Number:      number,
			Source:      fmt.Sprintf("prepdrive:%d", t.Data.Template.ID),
			QuestionIDs: ids,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out, nil
}
