package scheduler

import (
	"slices"
	"sync"
	"time"
)

const (
	// ApiRequestQuota per second
	ApiRequestQuota        = 30
	ApiRequestQuotaTimeout = time.Second

	// ApiRequestQuotaPerChat per minute
	ApiRequestQuotaPerChat        = 20
	ApiRequestQuotaPerChatTimeout = time.Minute

	DefaultPollingRate = time.Millisecond
)

type RawFunc func() ([]byte, error)

type Scheduler interface {
	SyncFunc(count int64, chat string, fn RawFunc) ([]byte, error)
}

var _ Scheduler = &scheduler{}

var _ Scheduler = &nilScheduler{}

func Nil() Scheduler {
	return &nilScheduler{}
}

func (sch *nilScheduler) SyncFunc(count int64, chat string, fn RawFunc) ([]byte, error) {
	return nil, nil
}

type nilScheduler struct{}

func Conservative() Scheduler {
	return Custom(ApiRequestQuota*4/5, ApiRequestQuotaPerChat*4/5, DefaultPollingRate*10)
}

func Default() Scheduler {
	return Custom(ApiRequestQuota, ApiRequestQuotaPerChat, DefaultPollingRate)
}

func Custom(global int64, perChat int64, pollingRate time.Duration) Scheduler {
	return &scheduler{
		globalLimit:  global,
		global:       0,
		perChatLimit: perChat,
		perChat:      map[string]int64{},
		sync:         &sync.RWMutex{},
		events:       []event{},
		pollingRate:  pollingRate,
	}
}

type scheduler struct {
	globalLimit int64
	global      int64

	perChatLimit int64
	perChat      map[string]int64

	sync        *sync.RWMutex
	events      []event
	pollingRate time.Duration
}

func (sch *scheduler) SyncFunc(count int64, chat string, fn RawFunc) (ret []byte, err error) {
	if sch == nil {
		ret, err = fn()
		return
	}

	ticker := time.NewTicker(sch.pollingRate)
	defer ticker.Stop()
	for now := time.Now(); true; now = <-ticker.C {
		sch.sync.Lock()
		sch.handleEvents(now)

		if !sch.isReadyFor(count, chat) {
			sch.sync.Unlock()
			continue
		}

		ret, err = fn()
		sch.add(count, chat)

		sch.sync.Unlock()
		break
	}

	return
}

//func (sch *scheduler) Sync(count int64, chat int64) {
//	_, _ = sch.SyncFunc(count, chat, func() ([]byte, error) { return nil, nil })
//}

func (sch *scheduler) isReadyFor(count int64, chat string) bool {
	if sch == nil {
		return true
	}

	if sch.globalLimit < sch.global+count {
		return false
	}
	if perChat, contains := sch.perChat[chat]; contains && sch.perChatLimit < perChat+count {
		return false
	}

	return true
}

type event struct {
	time  time.Time
	count int64
	chat  string
}

func (sch *scheduler) order() {
	slices.SortFunc(sch.events, func(lhs, rhs event) int {
		return lhs.time.Compare(rhs.time)
	})
}

func (sch *scheduler) add(count int64, chat string) {
	now := time.Now()

	sch.global += count
	sch.events = append(sch.events, event{
		time:  now.Add(ApiRequestQuotaTimeout),
		count: count,
		chat:  "",
	})

	if chat != "" {
		sch.perChat[chat] += count
		sch.events = append(sch.events, event{
			time:  now.Add(ApiRequestQuotaPerChatTimeout),
			count: count,
			chat:  chat,
		})
	}

	sch.order() // it's not the best implementation
}

func (sch *scheduler) handleEvents(now time.Time) {
	if sch == nil || len(sch.events) == 0 {
		return
	}

	handled := 0
	for _, event := range sch.events {
		if event.time.After(now) {
			break
		}

		handled += 1
		if event.chat == "" {
			sch.global -= event.count
			continue
		}

		sch.perChat[event.chat] -= event.count
		if sch.perChat[event.chat] <= 0 {
			delete(sch.perChat, event.chat)
		}
	}
	sch.events = sch.events[handled:]
}
