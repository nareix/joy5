package pktop

import (
	"time"

	"github.com/nareix/joy5/av"
)

type NativeRateLimiter struct {
	pktTimeStart  time.Duration
	wallTimeStart time.Time
}

func NewNativeRateLimiter() *NativeRateLimiter {
	return &NativeRateLimiter{
		wallTimeStart: time.Now(),
	}
}

func (l *NativeRateLimiter) Do(in []av.Packet) []av.Packet {
	for _, pkt := range in {
		if pktTimeDiff := pkt.Time - l.pktTimeStart; pktTimeDiff > 0 {
			wallTimeDiff := time.Now().Sub(l.wallTimeStart)
			if wallTimeDiff < pktTimeDiff {
				time.Sleep(pktTimeDiff - wallTimeDiff)
			}
			l.pktTimeStart = pkt.Time
			l.wallTimeStart = time.Now()
		}
	}
	return in
}
