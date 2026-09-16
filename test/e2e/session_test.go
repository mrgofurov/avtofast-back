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
