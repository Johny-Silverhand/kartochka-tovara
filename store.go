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

const MaxPhotos = 4

type Card struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Photos      []string  `json:"photos,omitempty"`
	PhotoFile   string    `json:"photoFile,omitempty"` // legacy single photo
	UpdatedAt   time.Time `json:"updatedAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (c *Card) normalizePhotos() {
	if len(c.Photos) == 0 && c.PhotoFile != "" {
		c.Photos = []string{c.PhotoFile}
		c.PhotoFile = ""
	}
	if c.Photos == nil {
		c.Photos = []string{}
	}
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
	if err := json.Unmarshal(b, &s.cards); err != nil {
		return err
	}
	changed := false
	for i := range s.cards {
		before := len(s.cards[i].Photos)
		legacy := s.cards[i].PhotoFile
		s.cards[i].normalizePhotos()
		if legacy != "" || before != len(s.cards[i].Photos) {
			changed = true
		}
	}
	if changed {
		return s.saveLocked()
	}
	return nil
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
	for i := range s.cards {
		s.cards[i].normalizePhotos()
		out[i] = s.cards[i]
		out[i].Photos = append([]string{}, s.cards[i].Photos...)
	}
	return out
}

func (s *Store) Get(id string) (Card, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.cards {
		if c.ID == id {
			c.normalizePhotos()
			c.Photos = append([]string{}, c.Photos...)
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
		Photos:      []string{},
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
			s.cards[i].normalizePhotos()
			if err := s.saveLocked(); err != nil {
				return Card{}, err
			}
			c := s.cards[i]
			c.Photos = append([]string{}, c.Photos...)
			return c, nil
		}
	}
	return Card{}, fmt.Errorf("not found")
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.cards {
		if c.ID == id {
			c.normalizePhotos()
			for _, f := range c.Photos {
				_ = os.Remove(filepath.Join(s.photoDir, f))
			}
			s.cards = append(s.cards[:i], s.cards[i+1:]...)
			return s.saveLocked()
		}
	}
	return fmt.Errorf("not found")
}

func (s *Store) AddPhoto(id, ext string, data []byte) (Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.cards {
		if s.cards[i].ID == id {
			s.cards[i].normalizePhotos()
			if len(s.cards[i].Photos) >= MaxPhotos {
				return Card{}, fmt.Errorf("max %d photos", MaxPhotos)
			}
			name := fmt.Sprintf("%s_%d%s", id, len(s.cards[i].Photos)+1, ext)
			// ensure unique
			for {
				path := filepath.Join(s.photoDir, name)
				if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
					break
				}
				name = fmt.Sprintf("%s_%s%s", id, newID()[:8], ext)
			}
			path := filepath.Join(s.photoDir, name)
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return Card{}, err
			}
			s.cards[i].Photos = append(s.cards[i].Photos, name)
			s.cards[i].PhotoFile = ""
			s.cards[i].UpdatedAt = time.Now().UTC()
			if err := s.saveLocked(); err != nil {
				return Card{}, err
			}
			c := s.cards[i]
			c.Photos = append([]string{}, c.Photos...)
			return c, nil
		}
	}
	return Card{}, fmt.Errorf("not found")
}

func (s *Store) RemovePhoto(id string, index int) (Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.cards {
		if s.cards[i].ID == id {
			s.cards[i].normalizePhotos()
			if index < 0 || index >= len(s.cards[i].Photos) {
				return Card{}, fmt.Errorf("bad index")
			}
			f := s.cards[i].Photos[index]
			_ = os.Remove(filepath.Join(s.photoDir, f))
			s.cards[i].Photos = append(s.cards[i].Photos[:index], s.cards[i].Photos[index+1:]...)
			s.cards[i].UpdatedAt = time.Now().UTC()
			if err := s.saveLocked(); err != nil {
				return Card{}, err
			}
			c := s.cards[i]
			c.Photos = append([]string{}, c.Photos...)
			return c, nil
		}
	}
	return Card{}, fmt.Errorf("not found")
}

func (s *Store) PhotoPath(file string) string {
	return filepath.Join(s.photoDir, filepath.Base(file))
}

func (s *Store) Root() string { return s.root }
