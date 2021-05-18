package client

import (
	"sync/atomic"
	"time"
)

type Breaker interface {
	Ready() bool
	Fail()
	Success()
}

var DefaultBreaker = NewSimpleBreaker(3, 10*time.Second)

type SimpleBreaker struct {
	lastFailureTime  time.Time
	failures         uint64
	failureThreshold uint64
	window           time.Duration
}

func NewSimpleBreaker(failThreshold uint64, window time.Duration) Breaker {
	return &SimpleBreaker{
		failureThreshold: failThreshold,
		window:           window,
	}
}

func (cb *SimpleBreaker) Ready() bool {
	if time.Since(cb.lastFailureTime) > cb.window {
		cb.reset()
		return true
	}

	failures := atomic.LoadUint64(&cb.failures)
	return failures < cb.failureThreshold
}

func (cb *SimpleBreaker) Success() {
	cb.reset()
}
func (cb *SimpleBreaker) Fail() {
	atomic.AddUint64(&cb.failures, 1)
	cb.lastFailureTime = time.Now()
}

func (cb *SimpleBreaker) reset() {
	atomic.StoreUint64(&cb.failures, 0)
	cb.lastFailureTime = time.Now()
}
