// Package state provides persistent state for the miner.
// Tracks committed/revealed challenges so restarts don't lose progress.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// CommitRecord tracks a pending commit-reveal pair.
type CommitRecord struct {
	ChallengeID string `json:"challenge_id"`
	Answer      string `json:"answer"`
	Salt        string `json:"salt"`
	CommitHash  string `json:"commit_hash"`
	CommitTx    string `json:"commit_tx"`
	CommitBlock int64  `json:"commit_block"`
	Revealed    bool   `json:"revealed"`
	RevealTx    string `json:"reveal_tx,omitempty"`
}

// Store persists miner state to a JSON file.
type Store struct {
	path    string
	mu      sync.Mutex
	Pending map[string]*CommitRecord `json:"pending"` // challenge_id → record
	Done    []string                 `json:"done"`     // completed challenge IDs (last 100)
}

// NewStore opens or creates a state store at the given path.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "miner_state.json")
	s := &Store{
		path:    path,
		Pending: make(map[string]*CommitRecord),
	}

	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, s)
		if s.Pending == nil {
			s.Pending = make(map[string]*CommitRecord)
		}
	}
	return s, nil
}

// HasCommitted returns true if this challenge has already been committed.
func (s *Store) HasCommitted(challengeID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Pending[challengeID]; ok {
		return true
	}
	for _, id := range s.Done {
		if id == challengeID {
			return true
		}
	}
	return false
}

// SaveCommit records a successful commit.
func (s *Store) SaveCommit(rec *CommitRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Pending[rec.ChallengeID] = rec
	return s.flush()
}

// GetPendingReveals returns all committed but unrevealed challenges.
func (s *Store) GetPendingReveals() []*CommitRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []*CommitRecord
	for _, rec := range s.Pending {
		if !rec.Revealed {
			result = append(result, rec)
		}
	}
	return result
}

// MarkRevealed marks a challenge as revealed.
func (s *Store) MarkRevealed(challengeID, txHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.Pending[challengeID]; ok {
		rec.Revealed = true
		rec.RevealTx = txHash
	}
	return s.flush()
}

// MarkDone moves a challenge from pending to done.
func (s *Store) MarkDone(challengeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Pending, challengeID)
	s.Done = append(s.Done, challengeID)
	if len(s.Done) > 100 {
		s.Done = s.Done[len(s.Done)-100:]
	}
	return s.flush()
}

func (s *Store) flush() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}
