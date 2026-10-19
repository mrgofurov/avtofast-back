-- 000006_add_road_signs.up.sql
-- Road signs as a reference: every sign of the Rules' appendix 1 with its
-- picture and title, grouped as the appendix groups them. The official
-- wording is not copied here — it is read from legal_paragraphs, so the
-- Rules stay the single source. Filled by cmd/import-signs.

CREATE TABLE IF NOT EXISTS road_signs (
    id          BIGSERIAL PRIMARY KEY,
    -- "2.4", "5.19.1"; empty for the few pictures that carry no number.
    code        VARCHAR(32) NOT NULL DEFAULT '',
    -- "warning", "priority", … in appendix order, with the heading shown.
    group_key   VARCHAR(32) NOT NULL,
    group_title TEXT NOT NULL,
    group_order INT NOT NULL,
    ordinal     INT NOT NULL,
    -- Title per locale: {"uz-Latn-UZ": "...", "uz-Cyrl-UZ": "...", "ru": "..."}.
    title       JSONB NOT NULL DEFAULT '{}'::jsonb,
    -- Relative to the media mount, like question media: "signs/<file>".
    image_url   TEXT NOT NULL,
    source      TEXT NOT NULL DEFAULT '',
    UNIQUE (image_url)
);

-- Questions whose wording or official explanation cites the sign, so a
-- learner can practise exactly the questions behind one sign.
CREATE TABLE IF NOT EXISTS road_sign_questions (
    sign_id     BIGINT NOT NULL REFERENCES road_signs(id) ON DELETE CASCADE,
    question_id BIGINT NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    PRIMARY KEY (sign_id, question_id)
);
