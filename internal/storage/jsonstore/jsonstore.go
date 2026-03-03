package jsonstore

import (
	"encoding/json"
	"os"
	"sync"

	"psv-crowd-counter/internal/core/models"
)

type JSONStore struct {
	path string
	mu   sync.Mutex
}

func New(path string) *JSONStore { return &JSONStore{path: path} }

func (s *JSONStore) Save(r models.Report) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var arr []models.Report
	f, err := os.Open(s.path)
	if err == nil {
		_ = json.NewDecoder(f).Decode(&arr)
		f.Close()
	}
	arr = append(arr, r)
	tmp, err := os.Create(s.path + ".tmp")
	if err != nil {
		return err
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(arr); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	return os.Rename(s.path+".tmp", s.path)
}

func (s *JSONStore) List() ([]models.Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var arr []models.Report
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return arr, nil
		}
		return nil, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&arr); err != nil {
		return nil, err
	}
	return arr, nil
}
