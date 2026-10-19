// Command classify-topics files every question under a topic of the syllabus
// (domain.Topics) by the clause its official explanation cites, using the
// Rules' text in legal_paragraphs to place clauses cited by band alone.
//
//	go run ./cmd/classify-topics            # classify and save
//	go run ./cmd/classify-topics -dry-run   # print the report only
//	go run ./cmd/classify-topics -v         # also list each question's topic
//
// Run it after every question import (and after cmd/lex-import, once, so the
// Rules are there to look bands up in). It is idempotent: questions already
// on the right topic are left untouched.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/avtofast/avtofast-back/internal/config"
	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/repository/postgres"
	"github.com/avtofast/avtofast-back/internal/usecase"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "print the report without saving")
	verbose := flag.Bool("v", false, "list every question with its topic")
	flag.Parse()

	cfg, _ := config.Load("config/config.yaml")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "classify-topics: database:", err)
		os.Exit(1)
	}
	defer db.Close()

	paragraphs, err := postgres.NewLegalRepository(db).ListParagraphs(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "classify-topics: rules:", err)
		os.Exit(1)
	}
	if len(paragraphs) == 0 {
		fmt.Fprintln(os.Stderr, "classify-topics: warning: legal_paragraphs is empty; run cmd/lex-import first, or band-only references cannot be placed")
	}
	classifier := usecase.NewTopicClassifier(paragraphs)

	repo := postgres.NewTopicRepository(db)
	questions, err := repo.ListQuestionTexts(ctx, domain.LocaleUzLatn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "classify-topics: questions:", err)
		os.Exit(1)
	}

	topics := map[string]string{}
	perTopic := map[string]int{}
	perHow := map[string]int{}
	changed := 0
	for _, q := range questions {
		topic, how := classifier.Classify(q.Prompt, q.Explanation)
		topics[q.PublicID] = topic
		perTopic[topic]++
		perHow[how]++
		if topic != q.Category {
			changed++
		}
		if *verbose || how == usecase.TopicByWording {
			ex := []rune(q.Explanation)
			if len(ex) > 90 {
				ex = ex[:90]
			}
			fmt.Printf("%-10s %-30s %-8s %s\n", q.PublicID, topic, how, string(ex))
		}
	}

	fmt.Printf("\n%d questions, %d change topic\n", len(questions), changed)
	hows := make([]string, 0, len(perHow))
	for h := range perHow {
		hows = append(hows, h)
	}
	sort.Strings(hows)
	for _, h := range hows {
		fmt.Printf("  filed by %-9s %d\n", h, perHow[h])
	}
	fmt.Println()
	for _, g := range domain.TopicGroups {
		fmt.Println(g.Title)
		for _, t := range domain.Topics {
			if t.Group == g.Key {
				fmt.Printf("  %4d  %s\n", perTopic[t.Key], t.Title)
			}
		}
	}

	if *dryRun {
		return
	}
	if err := repo.SetCategories(ctx, topics); err != nil {
		fmt.Fprintln(os.Stderr, "classify-topics: save:", err)
		os.Exit(1)
	}
	fmt.Printf("\nsaved %d questions\n", len(topics))
}
