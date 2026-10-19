-- 000004_add_legal_paragraphs.up.sql
-- Official texts the AI tutor answers from, one row per paragraph as lex.uz
-- publishes them: the Yo'l harakati qoidalari (VM 172-son qarori), the
-- "Yo'l harakati to'g'risida"gi Qonun and the traffic chapter of the
-- Ma'muriy javobgarlik to'g'risidagi kodeks. Filled by cmd/lex-import, which
-- replaces one document's rows on each run.

CREATE TABLE IF NOT EXISTS legal_paragraphs (
    id BIGSERIAL PRIMARY KEY,
    -- 'yhq', 'law' or 'mjtk'; document_title is how citations name it ("YHQ").
    document VARCHAR(32) NOT NULL,
    document_title TEXT NOT NULL,
    -- lex.uz element id ("-5955737"): links straight to the paragraph.
    source_element_id VARCHAR(32) NOT NULL,
    source_url TEXT NOT NULL,
    -- Position within the document; paragraphs of one clause are consecutive.
    ordinal INT NOT NULL,
    -- "16-bob. Tartibga solinmagan chorrahalar"; empty in appendices.
    chapter TEXT NOT NULL DEFAULT '',
    -- "1-ilova. Yo'l belgilari"; empty in the main text.
    appendix TEXT NOT NULL DEFAULT '',
    -- An appendix section ("2. Imtiyoz belgilari") or an article heading
    -- ("128³-modda. ... tezligini oshirib yuborishi").
    section TEXT NOT NULL DEFAULT '',
    -- "105", "2.4", "128³"; empty for introductory paragraphs.
    clause_number VARCHAR(64) NOT NULL DEFAULT '',
    text TEXT NOT NULL,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (document, source_element_id)
);
CREATE INDEX IF NOT EXISTS idx_legal_paragraphs_doc_ordinal ON legal_paragraphs(document, ordinal);
