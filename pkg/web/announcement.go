package web

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Announcement represents a public notice shown in the top bar.
type Announcement struct {
	ID        int64  `json:"id"`
	Level     string `json:"level"` // info, warn, fix
	Title     string `json:"title"`
	Content   string `json:"content"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
}

// AnnouncementStore persists announcements in a JSON file.
type AnnouncementStore struct {
	mu   sync.Mutex
	path string
	data []Announcement
}

// NewAnnouncementStore opens or creates the announcement file.
func NewAnnouncementStore(path string) (*AnnouncementStore, error) {
	s := &AnnouncementStore{path: path}
	raw, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(raw, &s.data)
	}
	if s.data == nil {
		s.data = []Announcement{}
	}
	return s, nil
}

func (s *AnnouncementStore) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0644)
}

// List returns all active announcements.
func (s *AnnouncementStore) List(activeOnly bool) []Announcement {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !activeOnly {
		return s.data
	}
	var out []Announcement
	for _, a := range s.data {
		if a.Active {
			out = append(out, a)
		}
	}
	return out
}

// Add creates a new announcement.
func (s *AnnouncementStore) Add(a *Announcement) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var maxID int64
	for _, item := range s.data {
		if item.ID > maxID {
			maxID = item.ID
		}
	}
	a.ID = maxID + 1
	a.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	if a.Level == "" {
		a.Level = "info"
	}
	s.data = append([]Announcement{*a}, s.data...)
	return s.save()
}

// Update modifies an existing announcement.
func (s *AnnouncementStore) Update(a *Announcement) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.data {
		if item.ID == a.ID {
			a.CreatedAt = item.CreatedAt
			s.data[i] = *a
			return s.save()
		}
	}
	return fmt.Errorf("announcement id %d not found", a.ID)
}

// Delete removes an announcement by ID.
func (s *AnnouncementStore) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.data {
		if item.ID == id {
			s.data = append(s.data[:i], s.data[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("announcement id %d not found", id)
}
