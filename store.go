package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Card struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	PhotoFile   string    `json:"photoFile,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Store struct {
	mu       sync.Mutex
	root     string
	cards    []Card
	dataFile string
	photoDir string
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func OpenStore(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	photoDir := filepath.Join(root, "photos")
	if err := os.MkdirAll(photoDir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{
		root:     root,
		dataFile: filepath.Join(root, "cards.json"),
		photoDir: photoDir,
		cards:    []Card{},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.dataFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s.saveLocked()
		}
		return err
	}
	if len(b) == 0 {
		s.cards = []Card{}
		return nil
	}
	return json.Unmarshal(b, &s.cards)
}

func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.cards, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.dataFile + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.dataFile)
}

func (s *Store) List() []Card {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Card, len(s.cards))
	copy(out, s.cards)
	return out
}

func (s *Store) Get(id string) (Card, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.cards {
		if c.ID == id {
			return c, true
		}
	}
	return Card{}, false
}

func (s *Store) Create(name, description string) (Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	c := Card{
		ID:          newID(),
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.cards = append(s.cards, c)
	if err := s.saveLocked(); err != nil {
		return Card{}, err
	}
	return c, nil
}

func (s *Store) Update(id, name, description string) (Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.cards {
		if s.cards[i].ID == id {
			s.cards[i].Name = name
			s.cards[i].Description = description
			s.cards[i].UpdatedAt = time.Now().UTC()
			if err := s.saveLocked(); err != nil {
				return Card{}, err
			}
			return s.cards[i], nil
		}
	}
	return Card{}, fmt.Errorf("not found")
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.cards {
		if c.ID == id {
			if c.PhotoFile != "" {
				_ = os.Remove(filepath.Join(s.photoDir, c.PhotoFile))
			}
			s.cards = append(s.cards[:i], s.cards[i+1:]...)
			return s.saveLocked()
		}
	}
	return fmt.Errorf("not found")
}

func (s *Store) SetPhoto(id, ext string, data []byte) (Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.cards {
		if s.cards[i].ID == id {
			if s.cards[i].PhotoFile != "" {
				_ = os.Remove(filepath.Join(s.photoDir, s.cards[i].PhotoFile))
			}
			name := id + ext
			path := filepath.Join(s.photoDir, name)
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return Card{}, err
			}
			s.cards[i].PhotoFile = name
			s.cards[i].UpdatedAt = time.Now().UTC()
			if err := s.saveLocked(); err != nil {
				return Card{}, err
			}
			return s.cards[i], nil
		}
	}
	return Card{}, fmt.Errorf("not found")
}

func (s *Store) PhotoPath(file string) string {
	return filepath.Join(s.photoDir, filepath.Base(file))
}

func (s *Store) Root() string { return s.root }
