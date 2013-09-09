package api

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-etcd/etcd"
	"github.com/pebbe/zmq3"
	"github.com/wakarimasenco/streamingchan/fourchan"
	"github.com/wakarimasenco/streamingchan/node"
	"github.com/wakarimasenco/streamingchan/version"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	PORT_NUMBER     = 4000
	MAX_CONNECTIONS = 64
)

type ApiConfig struct {
	CmdLine        string
	PortNumber     int
	Etcd           []string
	ClusterName    string
	MaxConnections int
}

type ApiServer struct {
	Stats         *node.NodeStats
	Config        ApiConfig
	stop          chan<- bool
	Etcd          *etcd.Client
	PostPubSocket *zmq3.Socket
	Listeners     []chan fourchan.Post
	Connections   int64
}

func (as *ApiServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	stats := map[string]interface{}{
		"posts_processed": map[string]interface{}{
			"5min": as.Stats.Aggregate(node.METRIC_POSTS, node.TIME_5SEC, 60),
			"1hr":  as.Stats.Aggregate(node.METRIC_POSTS, node.TIME_1MIN, 60),
			"1d":   as.Stats.Aggregate(node.METRIC_POSTS, node.TIME_1HOUR, 24),
		},
	}
	data := map[string]interface{}{
		"ok":              1,
		"revision":        version.GitHash,
		"build_date":      version.BuildDate,
		"cmd_line":        as.Config.CmdLine,
		"runtime":         time.Since(as.Stats.StartTime),
		"memory":          memStats.Alloc,
		"processId":       os.Getpid(),
		"posts_processed": as.Stats.Lifetime.Posts,
		"stats":           stats,
		"connections":     as.Connections,
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

func (as *ApiServer) commandHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if strings.Contains(r.URL.Path, "/commands/stop") {
		go func() { as.stop <- true }()
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

func (as *ApiServer) streamHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	values := r.URL.Query()
	filters := make([]Filter, 0, 16)
	for k, vs := range values {
		for _, v := range vs {
			switch k {
			case "board":
				filters = append(filters, Filter{Board: v})
			case "thread":
				spl := strings.Split(v, "/")
				if len(spl) != 2 {
					continue
				}
				b := spl[0]
				if t, e := strconv.Atoi(spl[1]); e == nil {
					filters = append(filters, Filter{Board: b, Thread: t})
				}
			case "comment":
				filters = append(filters, Filter{Comment: v})
			case "name":
				filters = append(filters, Filter{Name: v})
			case "trip":
				filters = append(filters, Filter{Trip: v})
			case "email":
				filters = append(filters, Filter{Email: v})
			}
		}
	}
	if as.Connections >= int64(as.Config.MaxConnections) && as.Config.MaxConnections != -1 {
		w.WriteHeader(400)
		p := map[string]interface{}{
			"ok":      0,
			"message": "Maximum connections reached.",
		}
		data, _ := json.Marshal(p)
		fmt.Fprint(w, string(data))
		return
	}

	atomic.AddInt64(&as.Connections, 1)
	defer atomic.AddInt64(&as.Connections, -1)

	subSocket, err := zmq3.NewSocket(zmq3.SUB)
	if err != nil {
		w.WriteHeader(500)
		p := map[string]interface{}{
			"ok":      0,
			"message": "Server Error",
		}
		data, _ := json.Marshal(p)
		fmt.Fprint(w, string(data))
		log.Print("Failed to create Sub Socket: ", err)
		return
	}
	resp, err := as.Etcd.Get(as.Config.ClusterName + "/nodes")
	if err != nil {
		w.WriteHeader(500)
		p := map[string]interface{}{
			"ok":      0,
			"message": "Server Error",
		}
		data, _ := json.Marshal(p)
		fmt.Fprint(w, string(data))
		log.Print("Failed to contact etcd: ", err)
		return
	}

	var nodes []node.NodeInfo
	result := resp[0]
	if e := json.Unmarshal([]byte(result.Value), &nodes); e != nil {
		w.WriteHeader(500)
		p := map[string]interface{}{
			"ok":      0,
			"message": "Server Error",
		}
		data, _ := json.Marshal(p)
		fmt.Fprint(w, string(data))
		log.Print("Failed to unmarshal nodes: ", err)
		return
	}

	for _, nodeInfo := range nodes {
		if err := subSocket.Connect(fmt.Sprintf("tcp://%s:%d", nodeInfo.Hostname, nodeInfo.PPubPort)); err != nil {
			log.Print(err)
		}
	}

	//stop := make(chan bool)
	updateNodes := make(chan *store.Response)
	updateNodeWatch := make(chan bool)

	go func(updateNodes chan *store.Response) {
		for response := range updateNodes {
			//log.Print("Found new node: " + response.Key)
			if err := subSocket.Connect(fmt.Sprintf("tcp://%s", response.Value)); err != nil {
				log.Print(err)
			}
		}
	}(updateNodes)

	go func(updateNodes chan *store.Response) {
		for {
			_, e := as.Etcd.Watch(as.Config.ClusterName+"/add-postpub/", 0, updateNodes, updateNodeWatch)
			if e.Error() == "User stoped watch" {
				break
			} else {
				log.Print("Error watching: ", e)
			}
		}
		close(updateNodes)
	}(updateNodes)

	subSocket.SetSubscribe("")
	w.WriteHeader(200)
	lastMessage := time.Now()
	for {
		if time.Now().Sub(lastMessage) > (30 * time.Second) {
			if _, err := fmt.Fprint(w, "\r\n"); err != nil {
				break
			}
		}
		data, err := subSocket.RecvBytes(zmq3.DONTWAIT)
		if err == syscall.EAGAIN {
			time.Sleep(1 * time.Millisecond)
			continue
		}
		counter = 0
		var post fourchan.Post
		dec := gob.NewDecoder(bytes.NewBuffer(data))
		err = dec.Decode(&post)
		if err != nil {
			log.Print("Failed to decode thread post ", err)
			continue
		}
		passed := true
		for _, filter := range filters {
			if !filter.Passes(post) {
				passed = false
				break
			}
		}
		if !passed {
			continue
		}
		jdata, _ := json.Marshal(post)
		d := strings.Replace(string(jdata), "\r\n", "\n", -1) + "\n"
		if _, err := fmt.Fprint(w, d); err != nil {
			break
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		as.Stats.Incr(node.METRIC_POSTS, 1)
		lastMessage := time.Now()
	}
	updateNodeWatch <- true
	subSocket.Close()
	//stop <- true
}

func NewApiServer(flags *FlagConfig, stop chan<- bool) *ApiServer {
	as := new(ApiServer)
	as.Config.CmdLine = strings.Join(os.Args, " ")
	as.Config.PortNumber = flags.HttpPort
	as.Config.MaxConnections = flags.MaxConnections
	as.Config.ClusterName = flags.ClusterName
	as.Config.Etcd = strings.Split(flags.Etcd, ",")
	as.stop = stop
	as.Stats = node.NewNodeStats()
	return as
}

func (as *ApiServer) Serve() error {
	log.Println("Starting HTTP Server on port", as.Config.PortNumber)

	as.Etcd = etcd.NewClient()
	if as.Config.Etcd != nil {
		if !as.Etcd.SetCluster(as.Config.Etcd) {
			log.Print("Failed to register to etcd.")
			return errors.New("ailed to register to etcd")
		}
	}

	http.HandleFunc("/status/", as.statusHandler)
	http.HandleFunc("/commands/", as.commandHandler)
	http.HandleFunc("/stream.json", as.streamHandler)
	if e := http.ListenAndServe(fmt.Sprintf(":%d", as.Config.PortNumber), nil); e != nil {
		log.Print("Error starting server", e)
		return e
	}
	return nil
}
