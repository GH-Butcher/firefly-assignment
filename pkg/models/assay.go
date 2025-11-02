package models

type Assay struct {
	Name     string   `json:"name"`
	TopWords []string `json:"top_words"`
}
