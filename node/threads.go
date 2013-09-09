package node

import (
	"github.com/wakarimasenco/streamingchan/fourchan"
	"time"
)

func (n *Node) ProcessThread(threadInfo fourchan.ThreadInfo) {
	defer n.RoutineWaitGroup.Done()
	n.RoutineWaitGroup.Add(1)
	n.Stats.Incr(METRIC_THREADS, 1)
	for tries := 0; tries < 3; tries++ {
		if thread, e := fourchan.DownloadThread(threadInfo.Board, threadInfo.No); e == nil {
			z := int64(0)
			for _, post := range thread.Posts {
				if post.Time > threadInfo.MinPost && post.Time <= threadInfo.LastModified {
					if n.PostPub == nil {
						return
					}
					post.MachineId = n.NodeId
					post.RangeMin = threadInfo.MinPost
					post.RangeMax = threadInfo.LastModified
					n.PostPub <- post
					z++
				}
			}
			n.Stats.Incr(METRIC_POSTS, z)
			return
		}
		time.Sleep(time.Duration(tries+1) * 500 * time.Millisecond)
	}
}
