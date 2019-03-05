package utils

import (
	"sync/atomic"
	"time"
)

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func AtomicStoreTime(p *int64, t time.Time) {
	if t.IsZero() {
		atomic.StoreInt64(p, 0)
	} else {
		atomic.StoreInt64(p, t.UnixNano())
	}
}

func AtomicLoadTime(p *int64) time.Time {
	v := atomic.LoadInt64(p)
	if v == 0 {
		return time.Time{}
	} else {
		return time.Unix(v/1e9, v%1e9)
	}
}
