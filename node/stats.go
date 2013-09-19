package node

import (
	"container/ring"
	"sync/atomic"
	"time"
)

const (
	METRIC_BOARD_REQUESTS = iota
	METRIC_THREADS        = iota
	METRIC_POSTS          = iota

	TIME_5SEC  = iota
	TIME_1MIN  = iota
	TIME_1HOUR = iota
)

type NodeStats struct {
	FiveSec   *ring.Ring
	OneMin    *ring.Ring
	OneHour   *ring.Ring
	Lifetime  NodeMetrics
	Rings     []**ring.Ring
	StartTime time.Time
}

type NodeMetrics struct {
	BoardRequests int64
	Threads       int64
	Posts         int64
}

func NewNodeStats() *NodeStats {
	ns := new(NodeStats)
	ns.FiveSec = ring.New(180)
	ns.OneMin = ring.New(120)
	ns.OneHour = ring.New(168)
	ns.Rings = []**ring.Ring{&ns.FiveSec, &ns.OneMin, &ns.OneHour}
	ns.StartTime = time.Now()
	transform := func(r *ring.Ring) {
		n := r
		for i := 0; i < r.Len(); i++ {
			nm := new(NodeMetrics)
			n.Value = nm
			n = n.Next()
		}
	}
	transform(ns.FiveSec)
	transform(ns.OneMin)
	transform(ns.OneHour)

	go func() {
		for counter := 1; ; counter++ {
			(ns.FiveSec.Value.(*NodeMetrics)).Zero()
			ns.FiveSec = ns.FiveSec.Move(1)
			if counter%12 == 0 {
				(ns.OneMin.Value.(*NodeMetrics)).Zero()
				ns.OneMin = ns.OneMin.Move(1)
			}
			if counter%720 == 0 {
				(ns.OneHour.Value.(*NodeMetrics)).Zero()
				ns.OneHour = ns.OneHour.Move(1)
			}
			time.Sleep(5 * time.Second)
		}
	}()
	return ns
}

func (nm *NodeMetrics) Zero() {
	nm.BoardRequests = 0
	nm.Threads = 0
	nm.Posts = 0
}

func (ns *NodeStats) Aggregate(metric int, ringCon int, count int) int64 {
	var r *ring.Ring
	switch ringCon {
	case TIME_5SEC:
		r = ns.FiveSec
	case TIME_1MIN:
		r = ns.OneMin
	case TIME_1HOUR:
		r = ns.OneHour
	}
	n := r
	v := int64(0)
	for i := 0; i < count; i++ {
		switch metric {
		case METRIC_BOARD_REQUESTS:
			v += (n.Value.(*NodeMetrics)).BoardRequests
		case METRIC_THREADS:
			v += (n.Value.(*NodeMetrics)).Threads
		case METRIC_POSTS:
			v += (n.Value.(*NodeMetrics)).Posts
		}
		n = n.Prev()
	}
	return v
}

func (ns *NodeStats) Incr(metric int, by int64) {
	for _, r := range ns.Rings {
		switch metric {
		case METRIC_BOARD_REQUESTS:
			atomic.AddInt64(&(((*r).Value.(*NodeMetrics)).BoardRequests), by)
		case METRIC_THREADS:
			atomic.AddInt64(&(((*r).Value.(*NodeMetrics)).Threads), by)
		case METRIC_POSTS:
			atomic.AddInt64(&(((*r).Value.(*NodeMetrics)).Posts), by)
		}
	}

	switch metric {
	case METRIC_BOARD_REQUESTS:
		atomic.AddInt64(&ns.Lifetime.BoardRequests, by)
	case METRIC_THREADS:
		atomic.AddInt64(&ns.Lifetime.Threads, by)
	case METRIC_POSTS:
		atomic.AddInt64(&ns.Lifetime.Posts, by)
	}
}
