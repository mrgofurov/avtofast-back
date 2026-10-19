package domain

import "context"

// Kinds of tutor source.
const (
	// TutorSourceRule is a clause of the Yo'l harakati qoidalari itself.
	TutorSourceRule = "rule"
	// TutorSourceQuestion is a question from the bank with its official
	// explanation, which quotes the clause it rests on.
	TutorSourceQuestion = "question"
)

// TutorPassage is one piece of official text the AI tutor may answer from.
type TutorPassage struct {
	// ID is unique across both kinds: "yhq:<lex.uz element id>" for rules,
	// the question's public id for questions.
	ID   string
	Kind string

	// Question sources only.
	QuestionID string
	Category   string
	Prompt     string
	Answer     string // text of the correct choice

	// Text is the official wording, verbatim: the clause for a rule, the
	// explanation for a question.
	Text string
	// Clause is how the source is cited. For a rule it names the chapter or
	// appendix and the clause number; for a question it is the reference its
	// explanation opens with ("YHQ 4-bobi 17-bandiga"), or empty.
	Clause string
	// URL links a rule to its paragraph on lex.uz.
	URL string
}

// TutorTurn is one earlier message of the chat, as the client replays it.
type TutorTurn struct {
	Role string `json:"role"` // "user" or "assistant"
	Text string `json:"text"`
}

type TutorModelRequest struct {
	Message string
	History []TutorTurn
	// Sources are numbered from 1 in the order given; the model cites them
	// by that number.
	Sources []TutorPassage
	// Focus, when set, is the question the learner just got wrong. It is
	// always Sources[0].
	Focus *TutorFocus
}

// TutorFocus is the learner's own attempt at a question, so the tutor can
// explain why their choice was wrong.
type TutorFocus struct {
	SelectedAnswer string
}

type TutorModelAnswer struct {
	Answer string
	// Grounded is the model's own claim that the answer rests on the sources.
	Grounded bool
	// UsedSources are the 1-based source numbers the answer relied on.
	UsedSources []int
}

// TutorModel is the language model behind the tutor.
type TutorModel interface {
	Answer(ctx context.Context, req TutorModelRequest) (*TutorModelAnswer, error)
}

// Official documents the tutor may quote, by LegalParagraph.Document.
const (
	DocumentRules = "yhq"  // Yo'l harakati qoidalari (VM 172-son qarori)
	DocumentLaw   = "law"  // "Yo'l harakati to'g'risida"gi Qonun (O'RQ-900)
	DocumentCode  = "mjtk" // Ma'muriy javobgarlik to'g'risidagi kodeks, XI bob
)

// LegalParagraph is one paragraph of an official text as lex.uz publishes it.
type LegalParagraph struct {
	Document string // DocumentRules, DocumentLaw or DocumentCode
	// DocumentTitle is the short name a citation starts with: "YHQ", "MJtK".
	DocumentTitle   string
	SourceElementID string // lex.uz element id ("-5955737")
	SourceURL       string
	Ordinal         int
	Chapter         string // "16-bob. Tartibga solinmagan chorrahalar"; empty in appendices
	Appendix        string // "1-ilova. Yo'l belgilari"; empty in the main text
	// Section is an appendix section ("2. Imtiyoz belgilari") or, in a law
	// or code, the article heading ("128³-modda. ... tezligini oshirib yuborishi").
	Section string
	// ClauseNumber is the band ("105"), sign ("2.4") or article ("128³");
	// empty for introductory paragraphs.
	ClauseNumber string
	Text         string
}

type LegalRepository interface {
	// ListParagraphs returns every document, each in document order.
	ListParagraphs(ctx context.Context) ([]LegalParagraph, error)
	// ReplaceDocument swaps one document's stored text for a fresh import,
	// atomically, leaving the other documents alone.
	ReplaceDocument(ctx context.Context, document string, paragraphs []LegalParagraph) error
}
