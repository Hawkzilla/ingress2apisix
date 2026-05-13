package web

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Feedback represents a user-submitted message.
type Feedback struct {
	ID        int64  `json:"id"`
	Category  string `json:"category"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Contact   string `json:"contact"`
	CreatedAt string `json:"created_at"`
}

// FeedbackStore persists feedback messages in a JSON file.
type FeedbackStore struct {
	mu   sync.Mutex
	path string
	data []Feedback
}

// NewFeedbackStore opens or creates the JSON feedback file.
func NewFeedbackStore(path string) (*FeedbackStore, error) {
	s := &FeedbackStore{path: path}
	data, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(data, &s.data)
	}
	if s.data == nil {
		s.data = []Feedback{}
	}
	return s, nil
}

func (s *FeedbackStore) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0644)
}

// Add inserts a new feedback message.
func (s *FeedbackStore) Add(fb *Feedback) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var maxID int64
	for _, item := range s.data {
		if item.ID > maxID {
			maxID = item.ID
		}
	}
	fb.ID = maxID + 1
	fb.CreatedAt = time.Now().Format("2006-01-02 15:04:05")

	s.data = append([]Feedback{*fb}, s.data...)
	return s.save()
}

// List returns feedback messages ordered by newest first, with the total count.
func (s *FeedbackStore) List(limit, offset int) ([]Feedback, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	total := len(s.data)
	if offset >= total {
		return []Feedback{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return s.data[offset:end], total, nil
}

// Delete removes a feedback message by ID.
func (s *FeedbackStore) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, item := range s.data {
		if item.ID == id {
			s.data = append(s.data[:i], s.data[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("feedback id %d not found", id)
}
