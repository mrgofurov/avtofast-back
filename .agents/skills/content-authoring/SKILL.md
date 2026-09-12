---
name: content-authoring
description: Workflows for question pack authoring, 4-locale validation, Ed25519 signing, and publish auditing.
---

# Content Authoring & Publishing Skill

Procedures for creating, validating, and publishing official driving theory question packs.

---

## 1. Required Locales Parity

Every question MUST have translations for all 4 supported locales:
- `uz-Latn-UZ` (Uzbek Latin - default)
- `uz-Cyrl-UZ` (Uzbek Cyrillic)
- `ru` (Russian)
- `en` (English)

Each locale translation must provide:
- `prompt`: Question text
- `choices`: At least 2 answer choices with `id`, `text`, and `position`
- `explanation`: Official explanation citing law/regulation reference

---

## 2. Validation & Publishing Flow

1. **Drafting**:
   - Content author creates pack draft via `POST /v1/admin/content/packs` (`Role: content_admin`).
   - Content author uploads questions via `POST /v1/admin/content/packs/:packId/questions`.
2. **Validation**:
   - Execute `POST /v1/admin/questions/:questionId/validate`.
   - Ensures no missing locales, choices count >= 2, correct choice is specified, and image SHA-256 checksum is present.
3. **Publishing**:
   - Publisher calls `POST /v1/admin/content/packs/:packId/publish` (`Role: content_publisher`).
   - Backend calculates manifest SHA-256 checksum and digitally signs it using Ed25519.
   - An immutable entry is written to `admin_audit_logs`.
   - Published pack is automatically exposed in `GET /v1/bootstrap` and `GET /v1/content/packs`.
