// Command import-signs fills the road sign reference from avtotestu.uz's
// sign list (data/avtotestu/belgilar.json): each sign's number, titles in
// three scripts and picture, which it keeps under uploads/signs. It also links
// every sign to the bank questions that cite it, so run it again after a
// question import.
//
//	curl -o data/avtotestu/belgilar.json https://www.avtotestu.uz/data/belgilar.json
//	go run ./cmd/import-signs
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/avtofast/avtofast-back/internal/config"
	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/repository/postgres"
)

const imageBase = "https://www.avtotestu.uz"

// The appendix's seven headings, in its order; the source lists them in the
// same order, so position decides the key.
var groupKeys = []string{"warning", "priority", "prohibitory", "mandatory", "information", "service", "additional"}

type sourceGroup struct {
	Title map[string]string `json:"title"`
	Items []struct {
		Src   string            `json:"src"`
		Code  string            `json:"code"`
		Title map[string]string `json:"title"`
	} `json:"items"`
}

func main() {
	src := flag.String("src", "./data/avtotestu/belgilar.json", "sign list JSON")
	uploadDir := flag.String("upload-dir", "./uploads", "media root; pictures go to <upload-dir>/signs")
	dryRun := flag.Bool("dry-run", false, "print the summary without saving")
	flag.Parse()

	b, err := os.ReadFile(*src)
	if err != nil {
		fail("read sign list", err)
	}
	var groups []sourceGroup
	if err := json.Unmarshal(b, &groups); err != nil {
		fail("parse sign list", err)
	}

	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		fail("config", err)
	}
	ctx := context.Background()
	db, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		fail("database", err)
	}
	defer db.Close()

	questions, err := postgres.NewTopicRepository(db).ListQuestionTexts(ctx, domain.LocaleUzLatn)
	if err != nil {
		fail("load questions", err)
	}

	if err := os.MkdirAll(filepath.Join(*uploadDir, "signs"), 0o755); err != nil {
		fail("create signs dir", err)
	}
	client := &http.Client{Timeout: 30 * time.Second}

	var signs []postgres.RoadSignImport
	linked, withQuestions := 0, 0
	for gi, g := range groups {
		key := fmt.Sprintf("group%d", gi+1)
		if gi < len(groupKeys) {
			key = groupKeys[gi]
		}
		for ii, it := range g.Items {
			file := filepath.Base(it.Src)
			if err := ensureImage(client, it.Src, filepath.Join(*uploadDir, "signs", file)); err != nil {
				fail("image "+it.Src, err)
			}
			code := strings.TrimSpace(it.Code)
			var ids []string
			if code != "" {
				m := newMention(code)
				for _, q := range questions {
					if m.in(q.Prompt) || m.in(q.Explanation) {
						ids = append(ids, q.PublicID)
					}
				}
			}
			if len(ids) > 0 {
				withQuestions++
				linked += len(ids)
			}
			signs = append(signs, postgres.RoadSignImport{
				Code:       code,
				GroupKey:   key,
				GroupTitle: g.Title["uz_lat"],
				GroupOrder: gi + 1,
				Ordinal:    ii + 1,
				Titles: map[string]string{
					domain.LocaleUzLatn: stripCode(it.Title["uz_lat"], code),
					domain.LocaleUzCyrl: stripCode(it.Title["uz_cyr"], code),
					domain.LocaleRu:     stripCode(it.Title["ru"], code),
				},
				ImageURL:    "signs/" + file,
				Source:      "avtotestu.uz",
				QuestionIDs: ids,
			})
		}
	}

	fmt.Printf("%d groups, %d signs; %d signs cited by questions (%d links)\n", len(groups), len(signs), withQuestions, linked)
	if *dryRun {
		return
	}
	if err := postgres.NewRoadSignRepository(db, postgres.NewContentRepository(db)).ReplaceSigns(ctx, signs); err != nil {
		fail("save", err)
	}
	fmt.Println("saved")
}

// stripCode drops the number the source puts in front of each title
// ("2.4 Yo'l bering" → "Yo'l bering"); the number is shown on its own.
func stripCode(title, code string) string {
	title = strings.TrimSpace(title)
	if code != "" && strings.HasPrefix(title, code) {
		title = strings.TrimSpace(strings.TrimPrefix(title, code))
	}
	return strings.TrimLeft(title, ".- ")
}

func ensureImage(client *http.Client, src, dest string) error {
	if st, err := os.Stat(dest); err == nil && st.Size() > 0 {
		return nil
	}
	req, err := http.NewRequest(http.MethodGet, imageBase+src, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Referer", imageBase+"/belgilar")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// A sign number alone is ambiguous — "1.1" is also a road marking and "2.4"
// a vehicle fault — so a question cites a sign only where the number sits
// next to the word "belgi": `3.27 "To'xtash taqiqlangan" yo'l belgisi`,
// `7.13 qo'shimcha-axborot belgisi`, `belgisi 5.19.1`.
const apos = `['ʻ‘’` + "`" + `]?`

var (
	afterSign = regexp.MustCompile(`(?i)^\.?(?:\s*[«"“„][^«»"“”„]{1,90}[»"”“])?\s*(?:-|–|—)?\s*` +
		`(?:(?:ogohlantiruvchi|imtiyoz\w*|taqiqlovchi|buyuruvchi|axborot[- ]ishora|axborot[- ]ko` + apos + `rsatgich|servis|qo` + apos + `shimcha[- ]axborot)\s+)?` +
		`(?:yo` + apos + `l\s+)?belgi`)
	beforeSign = regexp.MustCompile(`(?i)belgi\w*\s+$`)
)

type mention struct{ code string }

func newMention(code string) mention { return mention{code: code} }

// in reports whether text cites the sign: the exact number (not part of a
// longer one) with "belgi" right after it or right before it.
func (m mention) in(text string) bool {
	for start := 0; ; {
		i := strings.Index(text[start:], m.code)
		if i < 0 {
			return false
		}
		i += start
		end := i + len(m.code)
		start = end
		if i > 0 && (isDigit(text[i-1]) || text[i-1] == '.') {
			continue
		}
		if end < len(text) && isDigit(text[end]) {
			continue
		}
		if end+1 < len(text) && text[end] == '.' && isDigit(text[end+1]) {
			continue
		}
		if afterSign.MatchString(text[end:]) || beforeSign.MatchString(text[:i]) {
			return true
		}
	}
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", what, err)
	os.Exit(1)
}
