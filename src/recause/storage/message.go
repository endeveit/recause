package storage

import (
	"time"

	"github.com/endeveit/go-gelf/gelf"
)

// Custom structure needed only to convert GELF unix-timestamp to time.Time
type Message struct {
	Id           string                 `json:"id,omitempty"`
	Version      string                 `json:"version"`
	Host         string                 `json:"host"`
	ShortMessage string                 `json:"short_message"`
	FullMessage  string                 `json:"full_message,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
	Level        int32                  `json:"level,omitempty"`
	Facility     string                 `json:"facility,omitempty"`
	File         string                 `json:"file,omitempty"`
	Line         int32                  `json:"line,omitempty"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
}

// Returns custom message based on GELF message structure
func NewMessageFromGelf(msg *gelf.Message) *Message {
	return &Message{
		Version:      msg.Version,
		Host:         msg.Host,
		ShortMessage: msg.Short,
		FullMessage:  msg.Full,
		Timestamp:    time.Unix(int64(msg.TimeUnix), 0),
		Level:        msg.Level,
		Facility:     msg.Facility,
		File:         msg.File,
		Line:         msg.Line,
		Extra:        msg.Extra,
	}
}
