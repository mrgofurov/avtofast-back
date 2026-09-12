# AvtoFast — backend API release contract

**Audience:** backend, mobile, QA, and content-admin teams  
**Status:** implementation contract for the first production backend release  
**API base URL:** `https://api.<production-domain>/v1`  
**Transport:** HTTPS + JSON, UTF-8

This document specifies the server endpoints needed to replace the app's demo question repository. It is intentionally provider-neutral: Firebase Authentication or Supabase Authentication may issue the user token, but the API contract below remains the same.

## 1. Release scope

Release the **P0 endpoints** before connecting the Flutter client to production. P1 and P2 endpoints must be feature-flagged until their dependencies and moderation flows are ready.

| Priority | Release scope | Why it is needed |
| --- | --- | --- |
| P0 | Auth verification, profile/onboarding, versioned question packs, practice, mock exams, mistake review, dashboard, sync, entitlement verification, notification preferences | Required for a real learning product and the current client flows |
| P1 | Friends, leaderboards, challenges, push campaigns, TTS | Valuable product features; safe to release behind remote flags |
| P2 | AI mistake analysis, admin authoring UI | Requires billing enforcement, moderation, rate limits, and audit logs |

**Do not expose** correct answers in ordinary question-list responses. The server evaluates answers during online practice and exams. Offline packs are signed content packages because the device must evaluate answers while offline.

## 2. Shared conventions

### Authentication

All user endpoints require:

```http
Authorization: Bearer <verified Firebase-or-Supabase-JWT>
X-App-Version: 1.0.0
X-Platform: ios | android
X-Device-Id: <stable-installation-uuid>
Accept-Language: uz-Latn-UZ | uz-Cyrl-UZ | ru | en
```

The API must verify issuer, audience, signature, expiry, and provider subject before using a token. It must derive `userId` from the verified token; it must never accept `userId` from a request body.

Supported sign-in providers are Google, Apple, Telegram, phone, and email. The mobile client completes provider sign-in through Firebase/Supabase and sends the resulting access token to this API. A custom `/auth/login` endpoint is **not** required unless the selected auth provider requires one.

### Formats

- Timestamps are RFC 3339 UTC, e.g. `2026-09-09T12:30:00Z`.
- Dates are ISO 8601 dates, e.g. `2026-10-24`.
- Locales are `uz-Latn-UZ`, `uz-Cyrl-UZ`, `ru`, and `en`.
- IDs are opaque UUIDs or ULIDs. Do not encode business data in IDs.
- Money is integer minor units plus currency, e.g. `{ "amount": 2990000, "currency": "UZS" }`.
- Cursor pagination uses `cursor` and `limit` (default `20`, maximum `100`).
- All mutating calls accept `Idempotency-Key: <uuid>`. Store the key and return the original response for retries for at least 24 hours.

### Error envelope

Use this response shape for all non-2xx responses:

```json
{
  "error": {
    "code": "QUESTION_PACK_OUTDATED",
    "message": "Download the latest question pack before starting this exam.",
    "requestId": "req_01J...",
    "details": { "requiredVersion": "2026.09.1" }
  }
}
```

Use `400` for validation errors, `401` for absent/invalid credentials, `403` for entitlement/role denial, `404` for unavailable resources, `409` for stale content or duplicate state, `422` for a valid but unprocessable submission, and `429` for rate limiting. Include `Retry-After` for `429`.

### Feature flags

The app must read flags from `GET /bootstrap` and hide unavailable P1/P2 functionality. Do not use app-version checks as a substitute for a remote feature flag.

## 3. P0 endpoint inventory

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| GET | `/bootstrap` | optional | App configuration, active pack versions, flags, pass threshold |
| GET | `/me` | user | Profile, settings, onboarding state, entitlement summary |
| PATCH | `/me` | user | Avatar/profile metadata and privacy settings |
| PUT | `/me/onboarding` | user | Save source, knowledge level, language, target date, daily goal |
| PATCH | `/me/preferences` | user | Language, dark mode, notification and privacy preferences |
| GET | `/content/packs` | user | Pack catalogue and version availability |
| GET | `/content/packs/{packId}/manifest` | user | Signed metadata and delta/version information |
| GET | `/content/packs/{packId}/questions` | user | Online questions for a pack/category/locale |
| POST | `/content/packs/{packId}/offline-download` | user | Short-lived signed URL for an offline pack |
| POST | `/practice-sessions` | user | Create adaptive/category/mistake practice session |
| POST | `/practice-sessions/{sessionId}/answers` | user | Submit and evaluate one practice answer |
| POST | `/practice-sessions/{sessionId}/complete` | user | Finalize practice and award progress |
| GET | `/review/mistakes` | user | Scheduled incorrect questions for repeat study |
| POST | `/mock-exams` | user | Create an official-format timed mock exam |
| PUT | `/mock-exams/{examId}/answers/{questionId}` | user | Persist an in-progress answer; no feedback |
| POST | `/mock-exams/{examId}/submit` | user | Server-grade score, pass/fail, and detailed review |
| GET | `/mock-exams` | user | Paginated attempt history |
| GET | `/dashboard` | user | Streak, XP, readiness, goals, category performance |
| GET | `/analytics/progress` | user | Time series and question-level error history |
| POST | `/sync/events` | user | Idempotent offline progress upload |
| GET | `/sync/changes` | user | Cursor-based changes to merge on a device |
| GET | `/entitlements` | user | Premium entitlement and product availability |
| POST | `/billing/verify` | user | Server-side Apple/Google purchase verification |
| POST | `/billing/restore` | user | Reconcile purchases after restore action |
| PUT | `/devices/{deviceId}/push-token` | user | Register/revoke FCM/APNs token |
| PATCH | `/me/notification-preferences` | user | Reminder, streak, weekly-summary, result settings |

## 4. Bootstrap, profile, and onboarding

### `GET /bootstrap`

This endpoint can be cached for 15 minutes. It lets the app configure itself before the user begins a session.

```json
{
  "minimumSupportedAppVersion": "1.0.0",
  "defaultLocale": "uz-Latn-UZ",
  "supportedLocales": ["uz-Latn-UZ", "uz-Cyrl-UZ", "ru", "en"],
  "examRules": {
    "questionCount": 20,
    "durationSeconds": 1200,
    "passCorrectCount": 16
  },
  "activeQuestionPack": {
    "id": "uz-theory-2026-09",
    "version": "2026.09.1",
    "publishedAt": "2026-09-01T00:00:00Z"
  },
  "features": {
    "friends": false,
    "weeklyChallenges": false,
    "voiceExplanations": false,
    "aiMistakeAnalysis": false
  }
}
```

### `GET /me`

Return the user's durable state in one response so a new device can hydrate quickly.

```json
{
  "id": "usr_01J...",
  "displayName": "Mohira Karimova",
  "avatarUrl": null,
  "onboardingCompleted": true,
  "onboarding": {
    "acquisitionSource": "instagram",
    "knowledgeLevel": "beginner",
    "locale": "uz-Latn-UZ",
    "targetExamDate": "2026-10-24",
    "dailyQuestionGoal": 10
  },
  "preferences": {
    "locale": "uz-Latn-UZ",
    "theme": "system",
    "profileVisibility": "friends",
    "leaderboardVisibility": "friends"
  },
  "entitlement": {
    "tier": "free",
    "expiresAt": null
  }
}
```

### `PUT /me/onboarding`

```json
{
  "acquisitionSource": "instagram",
  "knowledgeLevel": "beginner",
  "locale": "uz-Latn-UZ",
  "targetExamDate": "2026-10-24",
  "dailyQuestionGoal": 10
}
```

Validation: `dailyQuestionGoal` must be between `1` and `100`; target date must not be before today. The server may return an initial personalized-plan summary, but the client must still be usable while that plan is recalculated.

## 5. Versioned question-bank and offline content

### Data model requirements

Each published question must retain these immutable or auditable fields:

```json
{
  "id": "sign-014",
  "packId": "uz-theory-2026-09",
  "contentVersion": "2026.09.1",
  "category": "road_signs",
  "difficulty": "easy",
  "image": {
    "url": "https://cdn.<domain>/questions/sign-014.webp",
    "sha256": "...",
    "alt": {
      "uz-Latn-UZ": "Sariq romb shaklidagi yo'l belgisi"
    }
  },
  "source": {
    "reference": "YHQ, 2.1 — Ustunlik belgilari",
    "effectiveFrom": "2026-09-01",
    "officialSourceUrl": "https://..."
  },
  "translations": {
    "uz-Latn-UZ": {
      "prompt": "Bu belgi nimani bildiradi?",
      "choices": [
        { "id": "a", "text": "...", "position": 1 },
        { "id": "b", "text": "...", "position": 2 }
      ],
      "explanation": "..."
    }
  },
  "status": "published"
}
```

The correct choice, explanation availability, and content status are server-controlled. Never mutate a published question in place: create a new content version, retain its source reference, and deprecate the old question/version with a reason.

Allowed categories are `road_signs`, `traffic_rules`, `intersections`, `first_aid`, `penalties`, `vehicle_safety`, and `situations`. Allowed difficulty values are `easy`, `medium`, and `hard`.

### Content endpoints

`GET /content/packs` returns each user's permitted packs and latest installed/version information:

```json
{
  "items": [
    {
      "id": "uz-theory-2026-09",
      "version": "2026.09.1",
      "title": "Uzbekiston haydovchilik nazariyasi",
      "locales": ["uz-Latn-UZ", "uz-Cyrl-UZ", "ru", "en"],
      "questionCount": 860,
      "downloadBytes": 18432000,
      "mandatoryUpdate": false
    }
  ]
}
```

`GET /content/packs/{packId}/questions?locale=uz-Latn-UZ&category=road_signs&cursor=...` returns question text/choices for online use. It must omit `correctChoiceId`.

`POST /content/packs/{packId}/offline-download` returns a short-lived URL only if the user may download the pack:

```json
{
  "url": "https://cdn.<domain>/signed/...",
  "expiresAt": "2026-09-09T13:00:00Z",
  "manifest": {
    "packId": "uz-theory-2026-09",
    "version": "2026.09.1",
    "sha256": "...",
    "signature": "base64-ed25519-signature"
  }
}
```

The offline ZIP contains all required locale content and answer keys, plus the signed manifest. The client verifies checksum/signature before activating a pack. Offline answer keys cannot be made secret on a user device; signing prevents tampering, not extraction.

## 6. Practice and mistake review

### `POST /practice-sessions`

Request:

```json
{
  "mode": "adaptive",
  "packId": "uz-theory-2026-09",
  "locale": "uz-Latn-UZ",
  "category": null,
  "questionCount": 10
}
```

`mode` is one of `adaptive`, `category`, `mistake_review`, or `daily_goal`. For adaptive mode, select weak categories and spaced-repetition items server-side. Return session metadata and ordered question objects **without answers**:

```json
{
  "id": "ps_01J...",
  "mode": "adaptive",
  "packVersion": "2026.09.1",
  "questions": [{ "id": "cross-027", "...": "question payload" }]
}
```

### `POST /practice-sessions/{sessionId}/answers`

Request:

```json
{
  "questionId": "cross-027",
  "selectedChoiceId": "a",
  "elapsedMs": 8200,
  "answeredAt": "2026-09-09T12:35:00Z"
}
```

Response:

```json
{
  "questionId": "cross-027",
  "isCorrect": true,
  "correctChoiceId": "a",
  "explanation": "Teng ahamiyatli yo'llar kesishmasida...",
  "reference": "YHQ, 15.2 — Chorrahalardan o'tish",
  "explanationAccess": "full",
  "xpAwarded": 12,
  "reviewScheduledAt": null
}
```

For a free user with an explanation limit, return `explanationAccess: "preview"` and either a safe preview or `null`; never lie about correctness. Incorrect questions must be automatically added/updated in the review queue using a spaced-repetition interval.

`POST /practice-sessions/{sessionId}/complete` finalizes score, goal progress, XP, streak changes, and category aggregates. It must be idempotent.

### `GET /review/mistakes`

Query parameters: `dueOnly=true|false`, `category`, `cursor`, and `limit`.

Each item returns the question payload plus review metadata:

```json
{
  "items": [
    {
      "question": { "id": "cross-027", "...": "question payload" },
      "mistakeCount": 3,
      "lastIncorrectAt": "2026-09-08T12:00:00Z",
      "nextReviewAt": "2026-09-10T09:00:00Z",
      "priority": "high"
    }
  ],
  "nextCursor": null
}
```

## 7. Timed mock exams

### `POST /mock-exams`

Request:

```json
{
  "packId": "uz-theory-2026-09",
  "locale": "uz-Latn-UZ",
  "format": "official_20"
}
```

The server selects the questions, fixes order and answer option order, records `packVersion`, and sets the deadline. Response:

```json
{
  "id": "exam_01J...",
  "status": "in_progress",
  "startedAt": "2026-09-09T12:40:00Z",
  "deadlineAt": "2026-09-09T13:00:00Z",
  "passCorrectCount": 16,
  "questions": [{ "id": "...", "...": "question payload" }]
}
```

### `PUT /mock-exams/{examId}/answers/{questionId}`

Persist an answer for resume/sync. Do not return correctness or explanations.

```json
{ "selectedChoiceId": "b", "elapsedMs": 10400 }
```

The server rejects submissions after deadline with `409 EXAM_EXPIRED`, then grades the attempt exactly once.

### `POST /mock-exams/{examId}/submit`

The server grades from its stored questions and answers. Do not accept client-supplied score, pass state, correct answer, or result list.

```json
{
  "status": "completed",
  "correctCount": 18,
  "questionCount": 20,
  "passCorrectCount": 16,
  "passed": true,
  "scorePercent": 90,
  "completedAt": "2026-09-09T12:57:00Z",
  "review": [
    {
      "questionId": "cross-027",
      "selectedChoiceId": "a",
      "correctChoiceId": "a",
      "isCorrect": true,
      "explanation": "...",
      "reference": "..."
    }
  ]
}
```

## 8. Dashboard, analytics, and synchronization

### `GET /dashboard`

Return one compact snapshot for the home screen:

```json
{
  "xp": 1240,
  "level": 6,
  "streakDays": 12,
  "dailyGoal": { "completed": 7, "target": 10 },
  "readinessScore": 72,
  "accuracyPercent": 78,
  "completedMockExams": 4,
  "weakCategories": [
    { "category": "intersections", "accuracyPercent": 58, "dueMistakes": 4 }
  ],
  "nextRecommendedAction": "mistake_review"
}
```

`GET /analytics/progress?range=30d` returns category accuracy, completed tests, readiness trend, error-history aggregates, and daily activity. Question-level history must only return the requesting user's data.

### Offline sync

The app writes local events while offline. It uploads them with `POST /sync/events`:

```json
{
  "events": [
    {
      "id": "evt_01J...",
      "type": "practice_answered",
      "occurredAt": "2026-09-09T12:35:00Z",
      "packId": "uz-theory-2026-09",
      "packVersion": "2026.09.1",
      "payload": {
        "questionId": "cross-027",
        "selectedChoiceId": "a",
        "elapsedMs": 8200
      }
    }
  ]
}
```

Return acceptance per event plus a sync cursor. Resolve conflicts server-side using event IDs, server-grade exams, and latest preference write; do not discard answer history merely because another device has newer settings.

`GET /sync/changes?cursor=<cursor>` returns profile changes, entitlement changes, review queue updates, updated dashboard aggregates, and a replacement cursor. Avoid returning the entire question bank through sync.

## 9. Premium, notifications, and device registration

### Entitlements and purchases

The client may display store products obtained through StoreKit/Google Play Billing, but it must unlock premium features only after the backend returns an active entitlement.

`GET /entitlements`:

```json
{
  "tier": "premium",
  "status": "active",
  "productId": "autofast.monthly",
  "expiresAt": "2026-10-09T12:00:00Z",
  "features": {
    "aiMistakeAnalysis": true,
    "advancedAnalytics": true,
    "voiceExplanations": true,
    "premiumMockExams": true,
    "ads": false
  }
}
```

`POST /billing/verify` accepts platform purchase evidence plus `platform` (`ios` or `android`) and verifies it with Apple/Google on the server. Store the original transaction/purchase token uniquely; never trust a client boolean such as `isPremium`.

`POST /billing/restore` repeats reconciliation for the authenticated user and returns the same entitlement shape. It supports the Restore Purchases button.

### Notifications

`PUT /devices/{deviceId}/push-token` request:

```json
{
  "platform": "ios",
  "pushToken": "fcm-or-apns-token",
  "appVersion": "1.0.0",
  "locale": "uz-Latn-UZ",
  "timezone": "Asia/Tashkent"
}
```

Passing `pushToken: null` revokes the token. `PATCH /me/notification-preferences` accepts booleans for `studyReminder`, `streakProtection`, `mistakeReview`, `weeklySummary`, and `scoreImprovement`, plus a local reminder time. The scheduler must honor the user's timezone, quiet hours, and opt-out state.

## 10. P1 social, voice, and P2 AI endpoints

Keep these endpoints disabled in `/bootstrap.features` until complete.

| Method | Path | Notes |
| --- | --- | --- |
| POST | `/friends/invitations` | Invite by a privacy-safe friend code or approved contact, never raw phone-number discovery by default |
| GET | `/friends` | List accepted friends only |
| POST | `/friends/invitations/{id}/accept` | Accept pending invite |
| DELETE | `/friends/{friendshipId}` | Remove/block relationship |
| GET | `/leaderboards/weekly` | Return only users who opted into the selected visibility scope |
| GET | `/challenges/current` | Weekly challenge details and eligibility |
| POST | `/challenges/{challengeId}/join` | Join an optional challenge |
| POST | `/speech/explanations` | Premium-only; returns a short-lived audio URL for question explanation and requested locale |
| POST | `/ai/mistake-analyses` | Premium-only; async analysis of aggregate mistake history, rate-limited |
| GET | `/ai/mistake-analyses/{analysisId}` | Pollable result or job status |

Social requests must enforce profile and leaderboard visibility server-side. AI requests must use server-held provider credentials, minimize personal data, record model/version metadata, and never send untrusted question content directly from the client to a third-party model.

## 11. Private content-admin API

Admin endpoints use a separate admin audience/role (`content_admin` or `content_publisher`) and must not be callable by the mobile app.

| Method | Path | Required role | Purpose |
| --- | --- | --- | --- |
| POST | `/admin/content/packs` | content_admin | Create a draft pack/version |
| POST | `/admin/content/packs/{packId}/questions` | content_admin | Add draft question, translations, image and source reference |
| PATCH | `/admin/questions/{questionId}` | content_admin | Edit draft content only |
| POST | `/admin/questions/{questionId}/validate` | content_admin | Validate every required locale, choices, source, and image |
| POST | `/admin/content/packs/{packId}/publish` | content_publisher | Publish immutable version, sign manifest, invalidate cache |
| POST | `/admin/content/packs/{packId}/rollback` | content_publisher | Switch active pack to a prior published version |
| GET | `/admin/audit-log` | content_publisher | Read content/publish/change audit entries |

Publishing must fail if any supported locale has missing prompt, choices, explanation, or reference; if a question has no correct choice; or if its asset checksum is missing. Every publish/rollback must write an actor, timestamp, reason, and before/after version to the audit log.

## 12. Flutter integration boundary

The current app exposes `QuestionRepository` in `lib/core/data/question_repository.dart`. Implement `ApiQuestionRepository` behind that interface first, then introduce API-backed repositories for profile, dashboard, exams, and entitlement state. Do not place HTTP calls in widgets.

Important mapping:

| Flutter model | Backend field |
| --- | --- |
| `TheoryQuestion.id` | `question.id` |
| `QuestionCategory` | `question.category` snake_case enum |
| `Difficulty` | `question.difficulty` |
| `QuestionTranslation` | `question.translations[locale]` |
| `hasIllustration` | `question.image != null` |
| `LearningStats` | `GET /dashboard` response |
| `OnboardingAnswers` | `PUT /me/onboarding` request |

For online sessions, remove `correctIndex` from the downloaded question payload and obtain feedback from `POST /practice-sessions/{sessionId}/answers`. For signed offline packs, map the locally signed correct choice to the existing model only after manifest verification.

## 13. Production release checklist

Before the backend is declared ready for mobile release:

- [ ] OpenAPI 3.1 specification matches every P0 method/path/schema in this document.
- [ ] Contract tests cover Flutter JSON fixtures for Uzbek Latin, Uzbek Cyrillic, Russian, and English.
- [ ] Auth verifier validates Firebase/Supabase issuer, audience, signature, expiration, and subject.
- [ ] Row-level authorization prevents cross-user reads and writes.
- [ ] Mock exams are server-graded and immutable after deadline/submission.
- [ ] Offline pack manifests are signed and checksum-verified; published content has an audit trail.
- [ ] Purchase verification runs server-side for both Apple and Google and has webhook/reconciliation handling.
- [ ] All P1/P2 features are disabled by default in `/bootstrap` until release-ready.
- [ ] Rate limits cover auth-sensitive, sync, AI, TTS, invite, and billing endpoints.
- [ ] Structured logs include `requestId`, endpoint, user pseudonymous ID, latency, and error code—never access tokens or answer text unnecessarily.
- [ ] Error tracking, database backups, migration rollback plan, and uptime/latency alerts are configured.
- [ ] Staging environment has a separate auth project, payment sandbox, signed content pack, and test push tokens.

