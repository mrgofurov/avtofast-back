// Command prepdrive-dump saves Prepdrive's study material — subjects, their
// topics and each topic's lesson blocks, the topic-test catalogue, road signs
// and test templates — as raw JSON under one directory.
//
// It only reads. Turning the dump into rows (topics, lessons, the
// question→topic mapping) is a separate import step, so the source can be
// re-examined without fetching it again and the importer can change freely.
//
//	go run ./cmd/prepdrive-dump -token "<bearer>" -out ./data/prepdrive
//
// Files already on disk are skipped unless -force is set, so an interrupted
// run picks up where it stopped. Pass -templates-from to also fetch numbered
// test templates (e.g. -templates-from 64 for the ones added since the last
// scrape); it stops at the first template that does not exist.
//
// -topic-questions additionally starts one topic-test attempt per topic to
// learn which questions belong to it. That is a write on the Prepdrive
// account (an attempt appears in its history), so it is off by default.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const apiBase = "https://cdn.prepdrive.uz/api/"

var errNotFound = errors.New("not found")

type client struct {
	http   *http.Client
	token  string
	lang   string
	out    string
	force  bool
	delay  time.Duration
	nFetch int
	nSkip  int
}

// fetch GETs (or POSTs) path and stores the body at out/<file>. It returns the
// decoded body either way, reading it back from disk when the file was kept.
func (c *client) fetch(method, path, file string) (map[string]any, error) {
	dest := filepath.Join(c.out, file)
	if !c.force && method == http.MethodGet {
		if b, err := os.ReadFile(dest); err == nil {
			c.nSkip++
			return decode(b)
		}
	}

	req, err := http.NewRequest(method, apiBase+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", c.lang)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Origin", "https://account.prepdrive.uz")
	req.Header.Set("Referer", "https://account.prepdrive.uz/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	time.Sleep(c.delay)

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, fmt.Errorf("401 on %s: token expired — copy a fresh Authorization header from account.prepdrive.uz", path)
	case resp.StatusCode == http.StatusNotFound:
		return nil, errNotFound
	case resp.StatusCode >= 300:
		return nil, fmt.Errorf("HTTP %d on %s: %.200s", resp.StatusCode, path, body)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(dest, body, 0o644); err != nil {
		return nil, err
	}
	c.nFetch++
	fmt.Printf("saved %s\n", file)
	return decode(body)
}

func decode(b []byte) (map[string]any, error) {
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("not JSON: %w", err)
	}
	return v, nil
}

// idsUnder returns the "id" of every object in the array at data.<key> — or
// at data itself when key is empty and data is an array.
func idsUnder(body map[string]any, key string) []string {
	var arr []any
	switch data := body["data"].(type) {
	case []any:
		if key == "" {
			arr = data
		}
	case map[string]any:
		if a, ok := data[key].([]any); ok {
			arr = a
		}
	}
	var ids []string
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch id := obj["id"].(type) {
		case float64:
			ids = append(ids, fmt.Sprintf("%d", int64(id)))
		case string:
			ids = append(ids, id)
		}
	}
	return ids
}

func main() {
	token := flag.String("token", os.Getenv("PREPDRIVE_TOKEN"), "Prepdrive bearer token (or PREPDRIVE_TOKEN)")
	out := flag.String("out", "./data/prepdrive", "directory to write JSON into")
	lang := flag.String("lang", "uz", "Accept-Language: uz, oz (cyrillic) or ru")
	force := flag.Bool("force", false, "re-download files that already exist")
	templatesFrom := flag.Int("templates-from", 0, "also fetch test-templates/<n>, <n+1>, … until one is missing (0 = skip)")
	topicQuestions := flag.Bool("topic-questions", false, "start one attempt per topic test to record its questions (writes to the Prepdrive account)")
	flag.Parse()

	if strings.TrimSpace(*token) == "" {
		fmt.Fprintln(os.Stderr, "missing -token (or PREPDRIVE_TOKEN)")
		os.Exit(2)
	}

	c := &client{
		http:  &http.Client{Timeout: 60 * time.Second},
		token: strings.TrimPrefix(strings.TrimSpace(*token), "Bearer "),
		lang:  *lang,
		out:   filepath.Join(*out, *lang),
		force: *force,
		delay: 300 * time.Millisecond,
	}

	if err := run(c, *templatesFrom, *topicQuestions); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("done: %d fetched, %d already on disk → %s\n", c.nFetch, c.nSkip, c.out)
}

func run(c *client, templatesFrom int, topicQuestions bool) error {
	// Lessons: subjects → topics → content blocks.
	subjects, err := c.fetch(http.MethodGet, "content/subjects", "content/subjects.json")
	if err != nil {
		return err
	}
	for _, sid := range idsUnder(subjects, "subjects") {
		subject, err := c.fetch(http.MethodGet, "content/subjects/"+sid, "content/subjects/"+sid+".json")
		if err != nil {
			return err
		}
		for _, tid := range idsUnder(subject, "topics") {
			if _, err := c.fetch(http.MethodGet, "content/topics/"+tid, "content/topics/"+tid+".json"); err != nil && !errors.Is(err, errNotFound) {
				return err
			}
		}
	}

	// Road signs, with their type groups.
	if _, err := c.fetch(http.MethodGet, "road-signs/types/all", "road-signs/types.json"); err != nil && !errors.Is(err, errNotFound) {
		return err
	}
	if _, err := c.fetch(http.MethodGet, "road-signs/all", "road-signs/all.json"); err != nil && !errors.Is(err, errNotFound) {
		return err
	}

	// Topic tests: the catalogue, and optionally each topic's questions.
	topicTests, err := c.fetch(http.MethodGet, "topic-tests", "topic-tests/index.json")
	if err != nil {
		return err
	}
	if topicQuestions {
		ids := idsUnder(topicTests, "topics")
		if len(ids) == 0 {
			ids = idsUnder(topicTests, "")
		}
		for _, tid := range ids {
			file := "topic-tests/" + tid + ".attempt.json"
			if _, err := os.Stat(filepath.Join(c.out, file)); err == nil && !c.force {
				c.nSkip++
				continue
			}
			if _, err := c.fetch(http.MethodPost, "topic-tests/"+tid+"/attempts/start", file); err != nil {
				return err
			}
		}
	}

	// Numbered test templates added since the last scrape.
	if templatesFrom > 0 {
		if _, err := c.fetch(http.MethodGet, "test-templates", "test-templates/index.json"); err != nil && !errors.Is(err, errNotFound) {
			return err
		}
		for n := templatesFrom; ; n++ {
			id := fmt.Sprintf("%d", n)
			body, err := c.fetch(http.MethodGet, "test-templates/"+id, "test-templates/"+id+".json")
			if errors.Is(err, errNotFound) {
				break
			}
			if err != nil {
				return err
			}
			// Past the last template the API may answer 200 with
			// success=false rather than 404; either way, that is the end.
			if ok, _ := body["success"].(bool); !ok || body["data"] == nil {
				_ = os.Remove(filepath.Join(c.out, "test-templates", id+".json"))
				break
			}
		}
	}
	return nil
}
