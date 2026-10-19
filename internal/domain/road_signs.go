package domain

import "context"

// RoadSign is one sign of the reference: its picture, number and title, the
// official wording of the Rules about it, and how many bank questions cite it.
type RoadSign struct {
	Code     string `json:"code"`
	Title    string `json:"title"`
	ImageURL string `json:"imageUrl"`
	// Description is the Rules' appendix 1 text for the sign, paragraph by
	// paragraph, verbatim; empty when the appendix has no entry for it.
	Description   []string `json:"description"`
	QuestionCount int      `json:"questionCount"`
}

// RoadSignGroup is one heading of appendix 1 ("Ogohlantiruvchi belgilar").
type RoadSignGroup struct {
	Key   string     `json:"key"`
	Title string     `json:"title"`
	Signs []RoadSign `json:"signs"`
}

// RoadSignRepository serves the road sign reference. Postgres only, filled
// by cmd/import-signs.
type RoadSignRepository interface {
	ListSigns(ctx context.Context, locale string) ([]RoadSignGroup, error)
	// SignQuestions returns the published questions that cite a sign.
	SignQuestions(ctx context.Context, code string) ([]*Question, error)
}
