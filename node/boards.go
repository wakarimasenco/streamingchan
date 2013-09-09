package node

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"github.com/wakarimasenco/streamingchan/fourchan"
	"log"
	"net/http"
	"strconv"
	"time"
)

func isEtcdError(e error) bool {
	switch e.(type) {
	case etcd.EtcdError:
		return true
	}
	return false
}

func (n *Node) checkBoardLock(board string, stop <-chan bool) bool {
	resp, err := n.EtcCluster.Get(n.Config.Cluster + "/boards-owner/" + board)
	if err == nil || (isEtcdError(err) && (err.(etcd.EtcdError)).ErrorCode == 100) {
		if err != nil {
			return true
		}
		if resp[0].Value == n.NodeId || resp[0].Value == "" || resp[0].Value == "none" {
			return true
		}
		log.Printf("Board %s is locked (by %s). Trying to unlock.", board, resp[0].Value)
		defer log.Printf("Done waiting for %s.", board)
		timeout := make(chan bool, 1)
		for tries := 0; tries < 5; tries++ {
			resp, err := n.EtcCluster.Get(n.Config.Cluster + "/boards-owner/" + board)
			if err == nil && (resp[0].Value == n.NodeId || resp[0].Value == "") {
				return true
			}
			go func() {
				time.Sleep(time.Duration(tries) * time.Second)
				timeout <- true
			}()
			select {
			case <-stop:
				return false
			case <-timeout:
				continue
			}
		}
		return true
	} else {
		log.Print("Failed to connect to etcd: ", err)
		return true
	}
}

func (n *Node) ProcessBoard(nodeIds []string, board string, stop <-chan bool) {
	defer n.RoutineWaitGroup.Done()
	n.RoutineWaitGroup.Add(1)
	if !n.checkBoardLock(board, stop) {
		return
	}
	message := 0
	resp, err := n.EtcCluster.Get(n.Config.Cluster + "/boards/" + board)
	if err == nil || (isEtcdError(err) && (err.(etcd.EtcdError)).ErrorCode == 100) {
		lastModified := 0
		var lastModifiedHeader time.Time
		if err == nil {
			if lastModified, err = strconv.Atoi(resp[0].Value); err != nil {
				lastModified = 0
			}
		}
		multiplier := 1
		maxMod := 0
		timeout := make(chan bool, 1)
		n.EtcCluster.Set(n.Config.Cluster+"/boards-owner/"+board, n.NodeId, 0)
		defer n.EtcCluster.TestAndSet(n.Config.Cluster+"/boards-owner/"+board, n.NodeId, "none", 0)
		for {
			oldLM := lastModified
			if threads, statusCode, lastModifiedStr, e := fourchan.DownloadBoard(board, lastModifiedHeader); e == nil {
				lastModifiedHeader, _ = time.Parse(http.TimeFormat, lastModifiedStr)
				//log.Print("LM : ", lastModified)
				for _, page := range threads {
					for _, thread := range page.Threads {
						if thread.LastModified > lastModified && lastModified != 0 {
							//var ti fourchan.ThreadInfo
							//ti = thread // copy
							//ti.Board = page.Board
							//ti.LastModified = lastModified
							thread.Board = page.Board
							thread.MinPost = lastModified
							thread.OwnerId = nodeIds[(message % len(nodeIds))]
							message++
							if n.ThreadPub == nil {
								return
							}
							//log.Printf("Sending %d to %s", thread.No, thread.OwnerId)
							n.ThreadPub <- thread
							multiplier = 1
						}
						if thread.LastModified > maxMod {
							maxMod = thread.LastModified
						}
					}
				}
				lastModified = maxMod
			} else if statusCode != 304 {
				log.Print("Error downloading board ", board, " ", e)
			}
			if oldLM != lastModified {
				n.EtcCluster.Set(n.Config.Cluster+"/boards/"+board, fmt.Sprintf("%d", lastModified), 0)
			}
			n.Stats.Incr(METRIC_BOARD_REQUESTS, 1)
			go func() {
				//log.Printf("Waiting %dms", 500*multiplier)
				time.Sleep(time.Duration(multiplier) * 500 * time.Millisecond)
				timeout <- true
			}()
			select {
			case <-stop:
				return
			case <-timeout:
				multiplier *= 2
			}
			if multiplier > 16 {
				multiplier = 16
			}
		}
	} else {
		log.Print("Failed to connect to etcd: ", err)
		return
	}
}
