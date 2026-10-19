// Package claude is the AI tutor's language model, backed by the Claude API.
package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/avtofast/avtofast-back/internal/domain"
)

const systemPrompt = `You are "AI Ustoz", the tutor inside Avtofast, an app that prepares learners in Uzbekistan for the driving-licence theory exam. Learners trust you to state the traffic rules (Yo'l harakati qoidalari, YHQ) exactly as they are written, so a wrong rule does them more harm than no answer.

Every learner message comes with numbered sources of two kinds, and they are the only facts you may use:
- type="rule": official legal text, verbatim, with where it sits in the ref attribute. It is one of: the Yo'l harakati qoidalari (YHQ, VM 172-son qarori); the "Yo'l harakati to'g'risida"gi Qonun; or the traffic chapter of the Ma'muriy javobgarlik to'g'risidagi kodeks (MJtK), which is where fines and other penalties are set. This is the law; it outranks everything else.
- type="question": a question from the official exam bank with its correct answer and official explanation, which quotes the clause it rests on. It shows how the exam asks about a rule.

- Answer only from the sources. Do not add rules, numbers, speeds, distances, fines, sign meanings or exam procedures from your own knowledge, even when you are confident: your memory of the YHQ may be outdated or wrong, and the learner cannot tell the difference.
- If the sources do not answer the question, set grounded to false and leave used_sources empty. Do not guess, and do not answer a nearby question instead. If they answer only part of it, answer that part and say plainly that the rest is not in your sources.
- When you state a rule, name the clause it comes from: for a rule source use its ref (for example "YHQ 105-band, 16-bob. Tartibga solinmagan chorrahalar" or "1-ilova, 2.4-belgi"); for a question source, the clause its explanation names. Base the rule on rule sources where one covers it.
- When the message includes the learner's attempt at a question, explain why their choice was wrong and why the correct answer is right, using that question's explanation.
- Write in Uzbek, Latin script, as a patient instructor talking to a learner. A short paragraph or a few short lines is usually enough. Plain text: no Markdown headings, tables or bold.
- Penalties come only from MJtK sources. State a fine exactly as the article does (for example "bazaviy hisoblash miqdorining besh baravari"); never convert it to so'm, because the BHM amount changes and is not in your sources.
- used_sources lists the number of every source your answer relies on.`

var outputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"answer":       map[string]any{"type": "string"},
		"grounded":     map[string]any{"type": "boolean"},
		"used_sources": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
	},
	"required":             []string{"answer", "grounded", "used_sources"},
	"additionalProperties": false,
}

type TutorModel struct {
	client anthropic.Client
	model  string
}

func NewTutorModel(apiKey, model string) *TutorModel {
	return &TutorModel{
		client: anthropic.NewClient(
			option.WithAPIKey(apiKey),
			option.WithRequestTimeout(60*time.Second),
			option.WithMaxRetries(2),
		),
		model: model,
	}
}

// escapeTags keeps bank text and the learner's message from opening or
// closing a tag. Quotes are left alone: Uzbek is full of apostrophes, and
// "bo&#39;lgan" is noise the model might copy into its answer.
var escapeTags = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace

// escapeAttr also escapes the double quote that would end the attribute.
var escapeAttr = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace

// renderSources lays the sources out as tagged blocks the prompt refers to
// by number.
func renderSources(req domain.TutorModelRequest) string {
	var b strings.Builder
	b.WriteString("<sources>\n")
	for i, s := range req.Sources {
		if s.Kind == domain.TutorSourceRule {
			fmt.Fprintf(&b, "<source id=\"%d\" type=\"rule\" ref=\"%s\">\n%s\n</source>\n",
				i+1, escapeAttr(s.Clause), escapeTags(s.Text))
			continue
		}
		fmt.Fprintf(&b, "<source id=\"%d\" type=\"question\">\n<question>%s</question>\n<correct_answer>%s</correct_answer>\n<official_explanation>%s</official_explanation>\n</source>\n",
			i+1, escapeTags(s.Prompt), escapeTags(s.Answer), escapeTags(s.Text))
	}
	b.WriteString("</sources>\n\n")
	if req.Focus != nil {
		b.WriteString("<learner_attempt>\nThe learner just answered source 1 incorrectly.")
		if req.Focus.SelectedAnswer != "" {
			fmt.Fprintf(&b, " They chose: %s", escapeTags(req.Focus.SelectedAnswer))
		}
		b.WriteString("\n</learner_attempt>\n\n")
	}
	fmt.Fprintf(&b, "<learner_message>\n%s\n</learner_message>", escapeTags(req.Message))
	return b.String()
}

func buildMessages(req domain.TutorModelRequest) []anthropic.BetaMessageParam {
	var msgs []anthropic.BetaMessageParam
	for _, t := range req.History {
		// The conversation must open with the learner; the client's greeting
		// bubble is not part of it.
		if len(msgs) == 0 && t.Role != "user" {
			continue
		}
		role := anthropic.BetaMessageParamRoleUser
		if t.Role == "assistant" {
			role = anthropic.BetaMessageParamRoleAssistant
		}
		msgs = append(msgs, anthropic.BetaMessageParam{
			Role:    role,
			Content: []anthropic.BetaContentBlockParamUnion{anthropic.NewBetaTextBlock(t.Text)},
		})
	}
	return append(msgs, anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(renderSources(req))))
}

func (m *TutorModel) Answer(ctx context.Context, req domain.TutorModelRequest) (*domain.TutorModelAnswer, error) {
	resp, err := m.client.Beta.Messages.New(ctx, anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(m.model),
		MaxTokens: 16000,
		System:    []anthropic.BetaTextBlockParam{{Text: systemPrompt}},
		Messages:  buildMessages(req),
		OutputConfig: anthropic.BetaOutputConfigParam{
			Effort: anthropic.BetaOutputConfigEffortMedium,
			Format: anthropic.BetaJSONOutputFormatParam{Schema: outputSchema},
		},
		// A safety classifier may decline a request; the server then retries
		// it on a fallback model inside the same call.
		Fallbacks: anthropic.BetaFallbacksParamOfDefault(),
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
	})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) {
			return nil, fmt.Errorf("claude: HTTP %d: %w", apiErr.StatusCode, err)
		}
		return nil, fmt.Errorf("claude: %w", err)
	}

	switch resp.StopReason {
	case anthropic.BetaStopReasonRefusal:
		// Declined even after fallback: treat as nothing to say.
		return &domain.TutorModelAnswer{}, nil
	case anthropic.BetaStopReasonMaxTokens:
		return nil, errors.New("claude: response hit max_tokens")
	}

	var text strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	var out struct {
		Answer      string `json:"answer"`
		Grounded    bool   `json:"grounded"`
		UsedSources []int  `json:"used_sources"`
	}
	if err := json.Unmarshal([]byte(text.String()), &out); err != nil {
		return nil, fmt.Errorf("claude: malformed structured output: %w", err)
	}
	return &domain.TutorModelAnswer{Answer: out.Answer, Grounded: out.Grounded, UsedSources: out.UsedSources}, nil
}
