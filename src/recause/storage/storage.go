package storage

import (
	"github.com/endeveit/go-gelf/gelf"
	"time"
)

type SearchQuery struct {
	Query  string    `json:"query"`
	Limit  int       `json:"limit,omitempty"`
	Offset int       `json:"offset,omitempty"`
	From   time.Time `json:"from,omitempty"`
	To     time.Time `json:"to,omitempty"`
}

type SearchResult struct {
	Total    int64     `json:"total"`
	TookMs   int64     `json:"took_ms"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
	Messages []Message `json:"messages"`
}

type Storage interface {
	GetMessage(string) (map[string]interface{}, error)
	GetMessages(*SearchQuery) (*SearchResult, error)
	HandleMessage(*gelf.Message)
	PeriodicFlush(chan bool)
	ValidateQuery(string) error
}
