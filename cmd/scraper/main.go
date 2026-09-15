package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"github.com/avtofast/avtofast-back/internal/repository/postgres"
	"github.com/avtofast/avtofast-back/pkg/logger"
)

// Prepdrive API response structures
type PrepdriveResponse struct {
	Success    bool          `json:"success"`
	Message    string        `json:"message"`
	StatusCode int           `json:"status_code"`
	Data       PrepdriveData `json:"data"`
}

type PrepdriveData struct {
	Template  TemplateInfo   `json:"template"`
	Questions []PrepQuestion `json:"questions"`
}

type TemplateInfo struct {
	ID             int    `json:"id"`
	ExternalID     string `json:"external_id"`
	Name           string `json:"name"`
	LanguageID     int    `json:"language_id"`
	QuestionsCount int    `json:"questions_count"`
}

type PrepQuestion struct {
	ID               int64             `json:"id"`
	ExternalID       int64             `json:"external_id"`
	LanguageID       int               `json:"language_id"`
	QuestionContent  []ContentItem     `json:"question_content"`
	ShuffleOptions   bool              `json:"shuffle_options"`
	Options          []PrepOption      `json:"options"`
	Images           []PrepImage       `json:"images"`
	Videos           []PrepVideo       `json:"videos"`
	Audios           []PrepAudio       `json:"audios"`
	Explanations     []PrepExplanation `json:"explanations"`
	CorrectOptionIDs []int64           `json:"correct_option_ids"`
}

type ContentItem struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	URL     string `json:"url,omitempty"`
}

type PrepOption struct {
	ID         int64  `json:"id"`
	OptionText string `json:"option_text"`
	IsCorrect  bool   `json:"is_correct"`
}

type PrepImage struct {
	ID        int64  `json:"id"`
	ImagePath string `json:"image_path"`
	ImageURL  string `json:"image_url"`
	WebpURL   string `json:"webp_url"`
	AltText   string `json:"alt_text"`
	MimeType  string `json:"mime_type"`
}

type PrepVideo struct {
	ID        int64  `json:"id"`
	VideoPath string `json:"video_path"`
	VideoURL  string `json:"video_url"`
	MimeType  string `json:"mime_type"`
}

type PrepAudio struct {
	ID        int64  `json:"id"`
	AudioPath string `json:"audio_path"`
	AudioURL  string `json:"audio_url"`
	MimeType  string `json:"mime_type"`
}

type PrepExplanation struct {
	ID   int64  `json:"id"`
	Text string `json:"text"`
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func detectCategory(prompt, explanation string) string {
	lower := strings.ToLower(prompt + " " + explanation)
	switch {
	case strings.Contains(lower, "belgi") || strings.Contains(lower, "belgisi") || strings.Contains(lower, "chiziq"):
		return domain.CategoryRoadSigns
	case strings.Contains(lower, "chorraha") || strings.Contains(lower, "harakatlanish") || strings.Contains(lower, "yo'l bering") || strings.Contains(lower, "imtiyoz") || strings.Contains(lower, "aylanma"):
		return domain.CategoryIntersections
	case strings.Contains(lower, "jarima") || strings.Contains(lower, "javobgarlik") || strings.Contains(lower, "bhm") || strings.Contains(lower, "jarimasi"):
		return domain.CategoryPenalties
	case strings.Contains(lower, "tibbiy") || strings.Contains(lower, "yordam") || strings.Contains(lower, "jarohat") || strings.Contains(lower, "qon") || strings.Contains(lower, "bog'lam"):
		return domain.CategoryFirstAid
	case strings.Contains(lower, "nosozlik") || strings.Contains(lower, "tormoz") || strings.Contains(lower, "shina") || strings.Contains(lower, "chiroq") || strings.Contains(lower, "tirkama"):
		return domain.CategoryVehicleSafety
	case strings.Contains(lower, "quvib o'tish") || strings.Contains(lower, "to'xtash") || strings.Contains(lower, "tezlik") || strings.Contains(lower, "masofa"):
		return domain.CategorySituations
	default:
		return domain.CategoryTrafficRules
	}
}

type Downloader struct {
	httpClient *http.Client
	uploadDir  string
	baseURL    string
	token      string
	cookie     string
	log        *logger.Logger
}

func (d *Downloader) downloadMedia(rawURL, subDir, defaultExt string) (localPath string, publicURL string, sha256Hex string, err error) {
	if rawURL == "" {
		return "", "", "", nil
	}

	ext := filepath.Ext(strings.Split(rawURL, "?")[0])
	if ext == "" {
		ext = defaultExt
	}

	uuidName := newUUID() + ext
	targetRelPath := filepath.Join("uploads", subDir, uuidName)
	targetDiskPath := filepath.Join(d.uploadDir, subDir, uuidName)

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64)")
	if d.cookie != "" {
		req.Header.Set("Cookie", d.cookie)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("HTTP %d downloading %s", resp.StatusCode, rawURL)
	}

	outFile, err := os.Create(targetDiskPath)
	if err != nil {
		return "", "", "", err
	}
	defer outFile.Close()

	hasher := sha256.New()
	mw := io.MultiWriter(outFile, hasher)

	_, err = io.Copy(mw, resp.Body)
	if err != nil {
		return "", "", "", err
	}

	sha256Hex = hex.EncodeToString(hasher.Sum(nil))
	publicURL = fmt.Sprintf("%s/%s", strings.TrimRight(d.baseURL, "/"), targetRelPath)

	return targetRelPath, publicURL, sha256Hex, nil
}

type TokenStore struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func saveTokens(acc, ref string) {
	b, _ := json.MarshalIndent(TokenStore{AccessToken: acc, RefreshToken: ref}, "", "  ")
	_ = os.WriteFile(".prepdrive_tokens.json", b, 0644)
}

func loadTokens() (string, string) {
	b, err := os.ReadFile(".prepdrive_tokens.json")
	if err != nil {
		return "", ""
	}
	var ts TokenStore
	if err := json.Unmarshal(b, &ts); err == nil {
		return ts.AccessToken, ts.RefreshToken
	}
	return "", ""
}

func refreshAccessToken(refreshToken string) (string, string, error) {
	url := "https://cdn.prepdrive.uz/api/auth/refresh-token"
	payload := map[string]string{"refresh_token": refreshToken}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "uz")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var res struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &res); err != nil {
		return "", "", fmt.Errorf("decode error: %w (body: %s)", err, string(respBody))
	}
	if !res.Success || res.Data.AccessToken == "" {
		return "", "", fmt.Errorf("%s (status: %d)", res.Message, resp.StatusCode)
	}

	saveTokens(res.Data.AccessToken, res.Data.RefreshToken)
	return res.Data.AccessToken, res.Data.RefreshToken, nil
}

func main() {
	tokenFlag := flag.String("token", "", "Prepdrive Bearer token (without 'Bearer ' prefix)")
	refreshTokenFlag := flag.String("refresh-token", "", "Prepdrive refresh token for automatic token rotation")
	cookieFlag := flag.String("cookie", "", "Prepdrive Cookie header string")
	startFlag := flag.Int("start", 1, "Template start ID (default: 1)")
	endFlag := flag.Int("end", 63, "Template end ID (default: 63)")
	uploadDirFlag := flag.String("upload-dir", "./uploads", "Base directory for uploads")
	baseURLFlag := flag.String("base-url", "https://api.avtofast.uz", "Public base URL for uploaded media")
	packIDFlag := flag.String("pack-id", "uz-theory-2026-09", "Question pack ID")
	skipMediaFlag := flag.Bool("skip-media", false, "Skip downloading media files and keep original URLs")
	flag.Parse()

	log := logger.DefaultLogger
	log.Info("=== Starting AvtoFast Question Importer / Scraper ===")

	// Load previously saved tokens if available
	savedAcc, savedRef := loadTokens()

	// Resolve token: flag > ENV > saved
	token := *tokenFlag
	if token == "" {
		token = os.Getenv("PREPDRIVE_TOKEN")
	}
	if token == "" {
		token = savedAcc
	}

	// Resolve refresh token: flag > ENV > saved
	refreshToken := *refreshTokenFlag
	if refreshToken == "" {
		refreshToken = os.Getenv("PREPDRIVE_REFRESH_TOKEN")
	}
	if refreshToken == "" {
		refreshToken = savedRef
	}

	cookie := *cookieFlag
	if cookie == "" {
		cookie = os.Getenv("PREPDRIVE_COOKIE")
	}

	// Prepare directories
	uploadDir := *uploadDirFlag
	for _, sub := range []string{"questions", "videos", "audios"} {
		dir := filepath.Join(uploadDir, sub)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Error(fmt.Sprintf("Failed to create directory %s", dir), err)
			os.Exit(1)
		}
	}

	// Load DB config
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Error("Failed to load config", err)
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		log.Error("Failed to connect to database", err)
		os.Exit(1)
	}
	defer db.Close()

	contentRepo := postgres.NewContentRepository(db)

	// Ensure pack exists
	pack, err := contentRepo.GetPackByID(ctx, *packIDFlag)
	if err != nil || pack == nil {
		now := time.Now().UTC()
		pack = &domain.QuestionPack{
			PackID:        *packIDFlag,
			Version:       "2026.09.1",
			Title:         "O'zbekiston haydovchilik nazariyasi (Prepdrive to'plami)",
			Locales:       []string{domain.LocaleUzLatn, domain.LocaleUzCyrl, domain.LocaleRu, domain.LocaleEn},
			QuestionCount: 0,
			DownloadBytes: 0,
			Status:        "published",
			PublishedAt:   &now,
		}
		if err := contentRepo.CreatePack(ctx, pack); err != nil {
			log.Error("Failed to create master question pack", err)
			os.Exit(1)
		}
		log.Info("Created master pack: " + pack.PackID)
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	downloader := &Downloader{
		httpClient: httpClient,
		uploadDir:  uploadDir,
		baseURL:    *baseURLFlag,
		token:      token,
		cookie:     cookie,
		log:        log,
	}

	totalSaved := 0
	totalImages := 0
	totalVideos := 0

	for templateID := *startFlag; templateID <= *endFlag; templateID++ {
		url := fmt.Sprintf("https://cdn.prepdrive.uz/api/test-templates/%d", templateID)
		log.Info(fmt.Sprintf("==> [%d/%d] Fetching template %d from %s", templateID, *endFlag, templateID, url))

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			log.Error(fmt.Sprintf("Failed to build request for template %d", templateID), err)
			continue
		}

		req.Header.Set("Accept", "application/json")
		req.Header.Set("Accept-Language", "uz")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			log.Error(fmt.Sprintf("Network request failed for template %d", templateID), err)
			continue
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Error(fmt.Sprintf("Failed to read response body for template %d", templateID), err)
			continue
		}

		if resp.StatusCode == http.StatusUnauthorized {
			if refreshToken != "" {
				log.Info("Token expired (401). Attempting automatic refresh...")
				newAcc, newRef, err := refreshAccessToken(refreshToken)
				if err == nil && newAcc != "" {
					token = newAcc
					if newRef != "" {
						refreshToken = newRef
					}
					log.Info("Successfully refreshed token! Retrying template...")
					templateID-- // Retry current template
					continue
				}
				log.Error("Failed refreshing token", err)
			}

			log.Error("Prepdrive API returned 401 UNAUTHORIZED. Please supply a valid -token and -cookie.", nil)
			fmt.Println("\n[ERROR]: Sizning Prepdrive Bearer tokeningiz eskirgan yoki noto'g'ri!")
			fmt.Println("Iltimos, Chrome DevTools (F12) -> Network bo'limidan cdn.prepdrive.uz so'rovi Authorization tokenini nusxalab quyidagicha ishga tushiring:")
			fmt.Println("go run cmd/scraper/main.go -token=\"YOUR_NEW_TOKEN\" -cookie=\"YOUR_COOKIE\"")
			os.Exit(1)
		}

		if resp.StatusCode != http.StatusOK {
			log.Error(fmt.Sprintf("Prepdrive API returned status %d for template %d: %s", resp.StatusCode, templateID, string(bodyBytes)), nil)
			continue
		}

		var prepResp PrepdriveResponse
		if err := json.Unmarshal(bodyBytes, &prepResp); err != nil {
			log.Error(fmt.Sprintf("Failed to parse JSON for template %d", templateID), err)
			continue
		}

		if !prepResp.Success || len(prepResp.Data.Questions) == 0 {
			log.Info(fmt.Sprintf("Template %d has no questions or success=false", templateID))
			continue
		}

		log.Info(fmt.Sprintf("Found %d questions in template %d. Processing...", len(prepResp.Data.Questions), templateID))

		for _, pq := range prepResp.Data.Questions {
			publicID := fmt.Sprintf("pd_%d", pq.ID)

			// Extract Prompt
			var promptParts []string
			var audioURL string
			for _, qc := range pq.QuestionContent {
				if qc.Content != "" {
					promptParts = append(promptParts, strings.TrimSpace(qc.Content))
				}
				if qc.Type == "audio" && qc.URL != "" {
					audioURL = qc.URL
				}
			}
			prompt := strings.Join(promptParts, "\n\n")

			// Extract Explanation
			var explParts []string
			for _, exp := range pq.Explanations {
				if exp.Text != "" {
					explParts = append(explParts, strings.TrimSpace(exp.Text))
				}
			}
			explanation := strings.Join(explParts, "\n\n")

			// Extract Options & Identify Correct Choice
			var choices []domain.ChoiceItem
			correctChoiceID := "a"

			for i, opt := range pq.Options {
				choiceID := fmt.Sprintf("%c", 'a'+i)
				choices = append(choices, domain.ChoiceItem{
					ID:       choiceID,
					Text:     strings.TrimSpace(opt.OptionText),
					Position: i + 1,
				})

				isCorrect := opt.IsCorrect
				for _, cid := range pq.CorrectOptionIDs {
					if cid == opt.ID {
						isCorrect = true
						break
					}
				}
				if isCorrect {
					correctChoiceID = choiceID
				}
			}

			// Download & Process Image
			var img *domain.QuestionImage
			if len(pq.Images) > 0 {
				primaryImg := pq.Images[0]
				imgSourceURL := primaryImg.WebpURL
				if imgSourceURL == "" {
					imgSourceURL = primaryImg.ImageURL
				}
				if imgSourceURL == "" {
					imgSourceURL = primaryImg.ImagePath
				}

				if imgSourceURL != "" {
					if *skipMediaFlag {
						img = &domain.QuestionImage{
							URL: imgSourceURL,
							Alt: map[string]string{domain.LocaleUzLatn: primaryImg.AltText},
						}
					} else {
						_, pubURL, sha, err := downloader.downloadMedia(imgSourceURL, "questions", ".webp")
						if err != nil {
							log.Error(fmt.Sprintf("Failed downloading image for %s: %s", publicID, imgSourceURL), err)
							img = &domain.QuestionImage{
								URL: imgSourceURL,
								Alt: map[string]string{domain.LocaleUzLatn: primaryImg.AltText},
							}
						} else {
							totalImages++
							img = &domain.QuestionImage{
								URL:    pubURL,
								SHA256: sha,
								Alt:    map[string]string{domain.LocaleUzLatn: primaryImg.AltText},
							}
						}
					}
				}
			}

			// Download & Process Video
			var videoURL string
			if len(pq.Videos) > 0 {
				primaryVideo := pq.Videos[0]
				vidSourceURL := primaryVideo.VideoURL
				if vidSourceURL == "" {
					vidSourceURL = primaryVideo.VideoPath
				}

				if vidSourceURL != "" {
					if *skipMediaFlag {
						videoURL = vidSourceURL
					} else {
						_, pubURL, _, err := downloader.downloadMedia(vidSourceURL, "videos", ".mp4")
						if err != nil {
							log.Error(fmt.Sprintf("Failed downloading video for %s", publicID), err)
							videoURL = vidSourceURL
						} else {
							totalVideos++
							videoURL = pubURL
						}
					}
				}
			}

			// Download & Process Audio
			var finalAudioURL string
			if len(pq.Audios) > 0 && pq.Audios[0].AudioURL != "" {
				audioURL = pq.Audios[0].AudioURL
			}
			if audioURL != "" {
				if *skipMediaFlag {
					finalAudioURL = audioURL
				} else {
					_, pubURL, _, err := downloader.downloadMedia(audioURL, "audios", ".mp3")
					if err != nil {
						finalAudioURL = audioURL
					} else {
						finalAudioURL = pubURL
					}
				}
			}

			category := detectCategory(prompt, explanation)

			// Construct Domain Question
			q := &domain.Question{
				PublicID:        publicID,
				PackTableID:     pack.ID,
				PackID:          pack.PackID,
				ContentVersion:  pack.Version,
				Category:        category,
				Difficulty:      domain.DifficultyEasy,
				Image:           img,
				VideoURL:        videoURL,
				AudioURL:        finalAudioURL,
				ExternalID:      pq.ID,
				Source:          &domain.QuestionSource{Reference: "YHQ (Prepdrive)"},
				CorrectChoiceID: correctChoiceID,
				Status:          "published",
				Translations: map[string]domain.QuestionTranslationData{
					domain.LocaleUzLatn: {
						Prompt:      prompt,
						Choices:     choices,
						Explanation: explanation,
					},
				},
			}

			if err := contentRepo.CreateQuestion(ctx, q); err != nil {
				log.Error(fmt.Sprintf("Failed saving question %s to DB", publicID), err)
			} else {
				totalSaved++
			}
		}

		time.Sleep(300 * time.Millisecond)
	}

	// Update question count in master pack
	pack.QuestionCount = totalSaved
	_ = contentRepo.UpdatePack(ctx, pack)

	log.Info(fmt.Sprintf("=== Scraping & Import Completed! ==="))
	log.Info(fmt.Sprintf("Total Questions Saved: %d", totalSaved))
	log.Info(fmt.Sprintf("Total Images Downloaded: %d", totalImages))
	log.Info(fmt.Sprintf("Total Videos Downloaded: %d", totalVideos))
}
