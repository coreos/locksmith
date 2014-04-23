package lock

import (
	"errors"
	"sort"
)

type semaphore struct {
	Index uint64 `json:"-"`
	Semaphore int `json:"semaphore"`
	Max int `json:"max"`
	Holders []string `json:"holders"`
}

func (s *semaphore) SetMax(max int) error {
	diff := s.Max - max

	s.Semaphore = s.Semaphore - diff
	s.Max = s.Max - diff
	
	return nil
}

func (s *semaphore) addHolder(h string) error {
	loc := sort.SearchStrings(s.Holders, h)
	switch {
	case loc == len(s.Holders):
		s.Holders = append(s.Holders, h)
	case s.Holders[loc] == h:
		return errors.New("Holder exists for this id")
	default:
		s.Holders = append(s.Holders[:loc], append([]string{h}, s.Holders[loc:]...)...)
	}

	return nil
}

func (s *semaphore) removeHolder(h string) error {
	loc := sort.SearchStrings(s.Holders, h)
	if s.Holders[loc] == h {
		s.Holders = append(s.Holders[:loc], s.Holders[loc+1:]...)
	} else {
		return errors.New("Lock not held.")
	}
	
	return nil
}

func (s *semaphore) Lock(h string) error {
	if s.Semaphore <= 0 {
		return errors.New("Semaphore is at 0")
	}

	if err := s.addHolder(h); err != nil {
		return err
	}

	s.Semaphore = s.Semaphore - 1

	return nil
}

func (s *semaphore) Unlock(h string) error {
	if err := s.removeHolder(h); err != nil {
		return err
	}

	s.Semaphore = s.Semaphore + 1

	return nil
}

func newSemaphore() (sem *semaphore) {
	return &semaphore{0, 1, 1, nil}
}

type holder struct {
	ID string `json:"-"`
	StartTime int64 `json:"startTime"`
}
