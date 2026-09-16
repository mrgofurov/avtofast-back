package e2e_test

import (
	"encoding/json"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sessions are the door to every other endpoint: without one the app can read
// /bootstrap and nothing else.
func TestE2E_Sessions(t *testing.T) {
	s := setupE2ETest(t)

	t.Run("POST /v1/auth/guest issues a usable session", func(t *testing.T) {
		resp := s.doRequest("POST", "/v1/auth/guest", map[string]any{
			"deviceId": "device-guest-e2e",
		}, "")
		require.Equal(t, fiber.StatusCreated, resp.StatusCode)

		var body map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.NotEmpty(t, body["accessToken"])
		assert.NotEmpty(t, body["refreshToken"])
		assert.Equal(t, "Bearer", body["tokenType"])

		user, _ := body["user"].(map[string]any)
		require.NotNil(t, user)
		assert.Equal(t, true, user["isGuest"])

		// The whole point of a guest session: authenticated endpoints work.
		me := s.doRequest("GET", "/v1/me", nil, body["accessToken"].(string))
		assert.Equal(t, fiber.StatusOK, me.StatusCode)
	})

	t.Run("POST /v1/auth/guest falls back to the X-Device-Id header", func(t *testing.T) {
		resp := s.doRequest("POST", "/v1/auth/guest", map[string]any{}, "")
		assert.Equal(t, fiber.StatusCreated, resp.StatusCode)
	})

	t.Run("the same device gets the same guest account back", func(t *testing.T) {
		first := s.doRequest("POST", "/v1/auth/guest", map[string]any{"deviceId": "device-stable"}, "")
		second := s.doRequest("POST", "/v1/auth/guest", map[string]any{"deviceId": "device-stable"}, "")

		var a, b map[string]any
		require.NoError(t, json.NewDecoder(first.Body).Decode(&a))
		require.NoError(t, json.NewDecoder(second.Body).Decode(&b))

		assert.Equal(t,
			a["user"].(map[string]any)["id"],
			b["user"].(map[string]any)["id"],
			"reopening the app must not create a second account",
		)
	})

	t.Run("POST /v1/auth/refresh renews an access token", func(t *testing.T) {
		start := s.doRequest("POST", "/v1/auth/guest", map[string]any{"deviceId": "device-refresh"}, "")
		var session map[string]any
		require.NoError(t, json.NewDecoder(start.Body).Decode(&session))

		resp := s.doRequest("POST", "/v1/auth/refresh", map[string]any{
			"refreshToken": session["refreshToken"],
		}, "")
		require.Equal(t, fiber.StatusOK, resp.StatusCode)

		var refreshed map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&refreshed))
		assert.NotEmpty(t, refreshed["accessToken"])
		assert.Equal(t,
			session["user"].(map[string]any)["id"],
			refreshed["user"].(map[string]any)["id"],
		)
	})

	t.Run("a refresh token is not accepted as a bearer token", func(t *testing.T) {
		start := s.doRequest("POST", "/v1/auth/guest", map[string]any{"deviceId": "device-typed"}, "")
		var session map[string]any
		require.NoError(t, json.NewDecoder(start.Body).Decode(&session))

		resp := s.doRequest("GET", "/v1/me", nil, session["refreshToken"].(string))
		assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode,
			"a 90-day refresh token must never work as a 24-hour access token")
	})

	t.Run("POST /v1/auth/refresh rejects a garbage token", func(t *testing.T) {
		resp := s.doRequest("POST", "/v1/auth/refresh", map[string]any{
			"refreshToken": "not-a-token",
		}, "")
		assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("POST /v1/auth/session refuses when Firebase is not configured", func(t *testing.T) {
		resp := s.doRequest("POST", "/v1/auth/session", map[string]any{
			"idToken": "anything",
		}, "")
		assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode,
			"an unverifiable token must never be trusted")
	})

	t.Run("POST /v1/auth/session requires an id token", func(t *testing.T) {
		resp := s.doRequest("POST", "/v1/auth/session", map[string]any{}, "")
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	})
}

// Deleting an account has to actually delete it — Guideline 5.1.1(v).
func TestE2E_DeleteAccount(t *testing.T) {
	s := setupE2ETest(t)

	start := s.doRequest("POST", "/v1/auth/guest", map[string]any{"deviceId": "device-doomed"}, "")
	var session map[string]any
	require.NoError(t, json.NewDecoder(start.Body).Decode(&session))
	token := session["accessToken"].(string)

	require.Equal(t, fiber.StatusOK, s.doRequest("GET", "/v1/me", nil, token).StatusCode)

	resp := s.doRequest("DELETE", "/v1/me", nil, token)
	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)

	after := s.doRequest("GET", "/v1/me", nil, token)
	assert.Equal(t, fiber.StatusUnauthorized, after.StatusCode,
		"the session must stop working, and must not silently recreate the account")

	// The deleted account must not come back under a new id either: a guest
	// session for the same device starts over from nothing.
	fresh := s.doRequest("POST", "/v1/auth/guest", map[string]any{"deviceId": "device-doomed"}, "")
	require.Equal(t, fiber.StatusCreated, fresh.StatusCode)
	var reborn map[string]any
	require.NoError(t, json.NewDecoder(fresh.Body).Decode(&reborn))
	assert.NotEqual(t,
		session["user"].(map[string]any)["id"],
		reborn["user"].(map[string]any)["id"],
	)
}

func TestE2E_NotificationPreferencesRoundTrip(t *testing.T) {
	s := setupE2ETest(t)

	patch := s.doRequest("PATCH", "/v1/me/notification-preferences", map[string]any{
		"studyReminder":    true,
		"streakProtection": false,
		"mistakeReview":    true,
		"weeklySummary":    false,
		"scoreImprovement": true,
		"reminderTime":     "20:00",
	}, s.UserToken)
	require.Equal(t, fiber.StatusOK, patch.StatusCode)

	get := s.doRequest("GET", "/v1/me/notification-preferences", nil, s.UserToken)
	require.Equal(t, fiber.StatusOK, get.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(get.Body).Decode(&body))
	assert.Equal(t, true, body["studyReminder"])
	assert.Equal(t, false, body["streakProtection"])
	assert.Equal(t, "20:00", body["reminderTime"])
}

// Work done offline has to count for what it would have counted for online.
func TestE2E_OfflineAnswersAreGradedAndCountedOnce(t *testing.T) {
	s := setupE2ETest(t)

	before := s.doRequest("GET", "/v1/dashboard", nil, s.UserToken)
	var initial struct {
		XP              int `json:"xp"`
		AccuracyPercent int `json:"accuracyPercent"`
	}
	require.NoError(t, json.NewDecoder(before.Body).Decode(&initial))

	// "sign-014" is answered by "a"; "cross-027" by "b". One right, one wrong.
	events := map[string]any{
		"events": []any{
			map[string]any{
				"id":         "evt_offline_1",
				"type":       "practice_answer",
				"occurredAt": "2026-09-16T09:00:00Z",
				"packId":     "uz-theory-2026-09",
				"payload": map[string]any{
					"questionId":       "sign-014",
					"selectedChoiceId": "a",
					"elapsedMs":        2500,
				},
			},
			map[string]any{
				"id":         "evt_offline_2",
				"type":       "practice_answer",
				"occurredAt": "2026-09-16T09:01:00Z",
				"packId":     "uz-theory-2026-09",
				"payload": map[string]any{
					"questionId":       "cross-027",
					"selectedChoiceId": "a",
					"elapsedMs":        4000,
				},
			},
		},
	}

	resp := s.doRequest("POST", "/v1/sync/events", events, s.UserToken)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var upload struct {
		ProcessedEvents []struct {
			ID       string `json:"id"`
			Accepted bool   `json:"accepted"`
			Reason   string `json:"reason"`
		} `json:"processedEvents"`
		SyncCursor string `json:"syncCursor"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&upload))
	require.Len(t, upload.ProcessedEvents, 2)
	for _, event := range upload.ProcessedEvents {
		assert.True(t, event.Accepted, event.Reason)
	}
	assert.NotEmpty(t, upload.SyncCursor)

	after := s.doRequest("GET", "/v1/dashboard", nil, s.UserToken)
	var credited struct {
		XP              int `json:"xp"`
		AccuracyPercent int `json:"accuracyPercent"`
		DueMistakeCount int `json:"dueMistakeCount"`
		StreakDays      int `json:"streakDays"`
	}
	require.NoError(t, json.NewDecoder(after.Body).Decode(&credited))

	assert.Greater(t, credited.XP, initial.XP, "offline answers earn XP")
	assert.Equal(t, 50, credited.AccuracyPercent, "one of two answered correctly")
	assert.Positive(t, credited.DueMistakeCount, "the wrong one joins the review queue")
	assert.Equal(t, 1, credited.StreakDays)

	// A client that never saw the first response uploads the same batch again.
	replay := s.doRequest("POST", "/v1/sync/events", events, s.UserToken)
	require.Equal(t, fiber.StatusOK, replay.StatusCode)

	final := s.doRequest("GET", "/v1/dashboard", nil, s.UserToken)
	var afterReplay struct {
		XP              int `json:"xp"`
		AccuracyPercent int `json:"accuracyPercent"`
	}
	require.NoError(t, json.NewDecoder(final.Body).Decode(&afterReplay))

	assert.Equal(t, credited.XP, afterReplay.XP,
		"a retried upload must not credit the same answers twice")
	assert.Equal(t, credited.AccuracyPercent, afterReplay.AccuracyPercent)
}

// An answer the client claims was correct is still graded by the server.
func TestE2E_OfflineAnswersAreNotTakenOnTrust(t *testing.T) {
	s := setupE2ETest(t)

	resp := s.doRequest("POST", "/v1/sync/events", map[string]any{
		"events": []any{
			map[string]any{
				"id":         "evt_liar_1",
				"type":       "practice_answer",
				"occurredAt": "2026-09-16T09:00:00Z",
				"payload": map[string]any{
					"questionId":       "cross-027",
					"selectedChoiceId": "a",
					// The client says it got this right. It did not.
					"isCorrect": true,
				},
			},
		},
	}, s.UserToken)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	after := s.doRequest("GET", "/v1/dashboard", nil, s.UserToken)
	var snapshot struct {
		AccuracyPercent int `json:"accuracyPercent"`
		DueMistakeCount int `json:"dueMistakeCount"`
	}
	require.NoError(t, json.NewDecoder(after.Body).Decode(&snapshot))

	assert.Zero(t, snapshot.AccuracyPercent, "the server grades, not the client")
	assert.Positive(t, snapshot.DueMistakeCount)
}
