package storage

import (
	"fmt"
	"os"
)

type Store struct {
	Base string
}

func New(base string) *Store {
	os.MkdirAll(base, 0755)
	return &Store{Base: base}
}

func (s *Store) Save(orderNum string, jobID int, data []byte) (string, error) {
	name := orderNum
	if name == "" {
		name = fmt.Sprintf("job_%d", jobID)
	}

	path := fmt.Sprintf("%s/%s.pdf", s.Base, name)
	return path, os.WriteFile(path, data, 0644)
}
