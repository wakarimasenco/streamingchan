package node

import (
	"encoding/json"
	"fmt"
	"github.com/wakarimasenco/streamingchan/version"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	PORT_NUMBER = 3333
)

type NodeServer struct {
	Node    *Node
	CmdLine string
	stop    chan<- bool
}

func (ns *NodeServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	stats := map[string]interface{}{
		"board_requests": map[string]interface{}{
			"5min": ns.Node.Stats.Aggregate(METRIC_BOARD_REQUESTS, TIME_5SEC, 60),
			"1hr":  ns.Node.Stats.Aggregate(METRIC_BOARD_REQUESTS, TIME_1MIN, 60),
			"1d":   ns.Node.Stats.Aggregate(METRIC_BOARD_REQUESTS, TIME_1HOUR, 24),
		},
		"threads_processed": map[string]interface{}{
			"5min": ns.Node.Stats.Aggregate(METRIC_THREADS, TIME_5SEC, 60),
			"1hr":  ns.Node.Stats.Aggregate(METRIC_THREADS, TIME_1MIN, 60),
			"1d":   ns.Node.Stats.Aggregate(METRIC_THREADS, TIME_1HOUR, 24),
		},
		"posts_published": map[string]interface{}{
			"5min": ns.Node.Stats.Aggregate(METRIC_POSTS, TIME_5SEC, 60),
			"1hr":  ns.Node.Stats.Aggregate(METRIC_POSTS, TIME_1MIN, 60),
			"1d":   ns.Node.Stats.Aggregate(METRIC_POSTS, TIME_1HOUR, 24),
		},
	}
	data := map[string]interface{}{
		"ok":                1,
		"boards":            ns.Node.OwnedBoards,
		"revision":          version.GitHash,
		"build_date":        version.BuildDate,
		"cmd_line":          ns.CmdLine,
		"runtime":           time.Since(ns.Node.Stats.StartTime),
		"memory":            memStats.Alloc,
		"processId":         os.Getpid(),
		"hostname":          ns.Node.Config.Hostname,
		"board_requests":    ns.Node.Stats.Lifetime.BoardRequests,
		"threads_processed": ns.Node.Stats.Lifetime.Threads,
		"posts_published":   ns.Node.Stats.Lifetime.Posts,
		"stats":             stats,
		"nodeidx":           ns.Node.LastNodeIdx,
	}
	//log.Print("Serving ", r.URL.Path)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	data["runtime"] = (data["runtime"].(time.Duration)).Seconds()
	out, err := json.Marshal(data)
	if err == nil {
		fmt.Fprint(w, string(out))
	}
}

func (ns *NodeServer) commandHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if strings.Contains(r.URL.Path, "/commands/stop") {
		go func() { ns.stop <- true }()
		p := map[string]interface{}{
			"ok":      1,
			"message": "Stop command sent, check logs",
		}
		data, _ := json.Marshal(p)
		fmt.Fprint(w, string(data))
		return
	} else {
		p := map[string]interface{}{
			"ok":      0,
			"message": "Invalid command",
		}
		data, _ := json.Marshal(p)
		fmt.Fprint(w, string(data))
		return
	}
}

func NewNodeServer(node *Node, stop chan<- bool) *NodeServer {
	ns := new(NodeServer)
	ns.Node = node
	ns.CmdLine = strings.Join(os.Args, " ")
	ns.stop = stop
	return ns
}

func (ns *NodeServer) Serve(portNumber int) {
	log.Println("Starting HTTP status server on port", portNumber)
	http.HandleFunc("/status/", ns.statusHandler)
	http.HandleFunc("/commands/", ns.commandHandler)
	http.ListenAndServe(fmt.Sprintf(":%d", portNumber), nil)
}
