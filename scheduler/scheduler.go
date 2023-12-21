package scheduler

type RawFunc func() ([]byte, error)

type Scheduler interface {
	SyncFunc(count int, chat string, fn RawFunc) ([]byte, error)
}

// Nil scheduler does nothing, performing all functions ASAP.
func Nil() Scheduler {
	return &nilScheduler{}
}

func (sch *nilScheduler) SyncFunc(count int, chat string, fn RawFunc) ([]byte, error) {
	return fn()
}

type nilScheduler struct{}

var _ Scheduler = &nilScheduler{}
