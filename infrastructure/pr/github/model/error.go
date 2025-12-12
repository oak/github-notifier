package model

type GHError struct {
	Message    string
	Path       []interface{}
	Extensions map[string]any
}
