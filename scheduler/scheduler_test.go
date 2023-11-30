package scheduler

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestConservative(t *testing.T) {
	//sch := Conservative()
	sch := Custom(40, 2, time.Millisecond*10)
	//var sch *scheduler = nil

	counter := atomic.Int64{}
	start := time.Now()
	for i := int64(1); i < 40; i++ {
		i := i
		go func() {
			for {
				sch.SyncFunc(1, i, func() {
					counter.Add(1)
					//fmt.Printf("%d.\t%d\n", counter.Load(), i)
				})
				time.Sleep(time.Millisecond * 100)
			}
		}()
	}
	fmt.Printf("%s\n\n", time.Now().Sub(start).String())

	time.Sleep(time.Second * 20)

	fmt.Printf("%d.\t\n", counter.Load())
}
