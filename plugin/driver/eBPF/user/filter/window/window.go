package window

import (
	"hades-ebpf/user/cache"
	"time"

	utilcache "k8s.io/apimachinery/pkg/util/cache"
	"k8s.io/utils/lru"
)

const (
	filterFlag        = 1
	filterDefaultTime = time.Hour
)

// Window interface defines the basic functions that a
// window filter should be implement
type IWindow interface {
	// basic check function for every window impletementation
	Check(string) bool
	// pre filter for struct inheritanced the Window
	Filter(string) bool
}

// Window for filters to count the specific field
type Window struct {
	quota    uint32
	duration time.Duration
	// internal fields
	cache *utilcache.LRUExpireCache
	// TODO: counter just for temp
	counter *lru.Cache
}

func NewWindow(quota uint32, duration time.Duration, size int) *Window {
	w := &Window{
		quota:    quota,
		duration: duration,
	}
	w.cache = utilcache.NewLRUExpireCacheWithClock(size, cache.GTicker)
	w.counter = lru.New(size)
	return w
}

// check the input, return true if it have not exceeded the quota
// As default, if it exceeds the quota, just cache this into filter
// for an hour.
// TODO: add the filter with exponential backoff
func (w *Window) Check(input string) bool {
	flag, ok := w.cache.Get(input)
	// have not cached, return true, also a timer should be added
	if !ok {
		w.cache.Add(input, 0, w.duration)
		w.counter.Add(input, 1)
		return true
	}
	// re-check with the flag
	if flag.(int) == filterFlag {
		return false
	}
	// it's not been filtered, just incr the count
	count, _ := w.counter.Get(input)
	// if exceed the quota, return false
	if count.(uint32) > w.quota {
		w.cache.Add(input, 1, filterDefaultTime)
		return false
	}
	w.counter.Add(input, count.(uint32)+1)
	// not, just incr the count, do not update time
	return true
}

// default filter function, nothing to do
func (w *Window) Filter(input string) bool {
	return false
}