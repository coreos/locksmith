package updateengine

import (
	"fmt"
)

type Status struct {
	LastCheckedTime int64
	Progress float64
	CurrentOperation string
	NewVersion string
	NewSize int64
}

func NewStatus(body []interface{}) (s Status) {
	s.LastCheckedTime = body[0].(int64)
	s.Progress = body[1].(float64)
	s.CurrentOperation = body[2].(string)
	s.NewVersion = body[3].(string)
	s.NewSize = body[4].(int64)
	
	return
}

func (s *Status) String() string {
	return fmt.Sprintf("LastCheckedTime=%v Progress=%v CurrentOperation=%q NewVersion=%v NewSize=%v",
		s.LastCheckedTime,
		s.Progress,
		s.CurrentOperation,
		s.NewVersion,
		s.NewSize,
	)
}


