package claude

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleRequest() domain.TutorModelRequest {
	return domain.TutorModelRequest{
		Message: "Nega <b>bu</b> noto'g'ri?",
		History: []domain.TutorTurn{
			{Role: "assistant", Text: "Assalomu alaykum!"},
			{Role: "user", Text: "Chorraha haqida"},
			{Role: "assistant", Text: "Qaysi chorraha?"},
		},
		Sources: []domain.TutorPassage{{
			ID: "pd_1", Kind: domain.TutorSourceQuestion, QuestionID: "pd_1", Prompt: "Kim birinchi o'tadi?", Answer: "O'ngdagi",
			Text: `YHQ 16-bobi 105-bandiga asosan, "o'ng" qoida.`,
		}, {
			ID: "yhq:-5955737", Kind: domain.TutorSourceRule, Clause: `YHQ 16-bob. "Tartibga" › 105-band`,
			Text: "105. Teng ahamiyatga ega <yo'llar>.",
		}},
		Focus: &domain.TutorFocus{SelectedAnswer: "Chapdagi"},
	}
}

func TestBuildMessages_DropsLeadingGreetingAndAppendsSources(t *testing.T) {
	msgs := buildMessages(sampleRequest())
	require.Len(t, msgs, 3)
	assert.Equal(t, anthropic.BetaMessageParamRoleUser, msgs[0].Role)
	assert.Equal(t, anthropic.BetaMessageParamRoleAssistant, msgs[1].Role)
	assert.Equal(t, anthropic.BetaMessageParamRoleUser, msgs[2].Role)

	rendered := renderSources(sampleRequest())
	assert.Contains(t, rendered, `<source id="1" type="question">`)
	assert.Contains(t, rendered, `<source id="2" type="rule" ref="YHQ 16-bob. &quot;Tartibga&quot; › 105-band">`+"\n105. Teng ahamiyatga ega &lt;yo'llar&gt;.")
	assert.Contains(t, rendered, "They chose: Chapdagi")
	// Bank text and the learner's message cannot open or close tags.
	assert.Contains(t, rendered, "&lt;b&gt;bu&lt;/b&gt;")
	assert.Contains(t, rendered, `"o'ng" qoida`)
}

func TestAnswer_SendsGroundedRequestAndParsesStructuredOutput(t *testing.T) {
	var got map[string]any
	var beta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		beta = r.Header.Get("anthropic-beta")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id": "msg_1", "type": "message", "role": "assistant", "model": "claude-opus-5-5",
			"stop_reason": "end_turn",
			"content": [{"type": "text", "text": "{\"answer\":\"O'ngdagiga yo'l berasiz.\",\"grounded\":true,\"used_sources\":[1]}"}],
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`)
	}))
	defer srv.Close()

	m := NewTutorModel("test-key", "claude-opus-5-5")
	m.client = anthropic.NewClient(option.WithAPIKey("test-key"), option.WithBaseURL(srv.URL), option.WithMaxRetries(0))

	ans, err := m.Answer(context.Background(), sampleRequest())
	require.NoError(t, err)
	assert.True(t, ans.Grounded)
	assert.Equal(t, []int{1}, ans.UsedSources)
	assert.Equal(t, "O'ngdagiga yo'l berasiz.", ans.Answer)

	assert.Equal(t, "claude-opus-5-5", got["model"])
	assert.Equal(t, "default", got["fallbacks"])
	assert.True(t, strings.Contains(beta, "server-side-fallback-2026-07-01"), beta)
	_, hasThinking := got["thinking"]
	assert.False(t, hasThinking, "Opus 5.5 thinks adaptively by default; disabling it is a 400")
	oc := got["output_config"].(map[string]any)
	assert.Equal(t, "medium", oc["effort"])
	assert.Equal(t, "json_schema", oc["format"].(map[string]any)["type"])
}

func TestAnswer_RefusalIsNotGrounded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"msg_2","type":"message","role":"assistant","model":"claude-opus-5-5",
			"stop_reason":"refusal","content":[],"usage":{"input_tokens":1,"output_tokens":0}}`)
	}))
	defer srv.Close()

	m := NewTutorModel("test-key", "claude-opus-5-5")
	m.client = anthropic.NewClient(option.WithAPIKey("test-key"), option.WithBaseURL(srv.URL), option.WithMaxRetries(0))
	ans, err := m.Answer(context.Background(), sampleRequest())
	require.NoError(t, err)
	assert.False(t, ans.Grounded)
}
