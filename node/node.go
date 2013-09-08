package node

import (
	"bytes"
	"crypto/md5"
	crand "crypto/rand"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-etcd/etcd"
	"github.com/pebbe/zmq3"
	"github.com/wakarimasenco/streamingchan/fourchan"
	"io"
	"log"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	THRD_PUB_PORT_LOW  = 40000
	THRD_PUB_PORT_HIGH = 50000
	POST_PUB_PORT_LOW  = 30000
	POST_PUB_PORT_HIGH = 40000
	THREAD_WORKERS     = 10
)

type NodeInfo struct {
	NodeId    string `json:"nodeid"`
	NodeIndex int    `json:"nodeindex"`
	Hostname  string `json:"hostname"`
	TPubPort  int    `json:"thread_pub_port"`
	PPubPort  int    `json:"post_pub_port"`
}

type NodeConfig struct {
	Etcd          []string
	Hostname      string
	BindIp        string
	OnlyBoards    []string
	ExcludeBoards []string
	Cluster       string
	ThreadWorkers int
}

type Node struct {
	NodeId           string
	Stats            *NodeStats
	Boards           *fourchan.Boards
	Config           NodeConfig
	EtcCluster       *etcd.Client
	ThreadPubSocket  *zmq3.Socket
	ThreadPub        chan fourchan.ThreadInfo
	ThreadSubSocket  *zmq3.Socket
	ThreadSub        chan fourchan.ThreadInfo
	PostPubSocket    *zmq3.Socket
	PostPub          chan fourchan.Post
	NewNodeWatch     chan bool
	UpdateNodeWatch  chan bool
	BoardStop        []chan bool
	Shutdown         bool
	Closed           bool
	SocketWaitGroup  sync.WaitGroup
	RoutineWaitGroup sync.WaitGroup
	OwnedBoards      []string
	LastNodeIdx      int
	NodeCount        int
	DivideMutex      sync.Mutex
}

func randNodeId() string {
	h := md5.New()

	b := make([]byte, 1024)
	n, err := io.ReadFull(crand.Reader, b)
	if n != len(b) || err != nil {
		panic(err)
		return ""
	}
	if sz, err := h.Write(b); err != nil {
		panic(err)
		return ""
	} else {
		if sz != n {
			panic("Failed to write random bytes to hash?")
		}
		return hex.EncodeToString(h.Sum(nil))
	}
	panic("")
}

func randInt(min, max int) int {
	return int(rand.Float64()*float64(max-min)) + min
}

func (n *Node) Close() {
	n.Closed = true
	if n.NewNodeWatch != nil {
		n.NewNodeWatch <- true
	}
	if n.UpdateNodeWatch != nil {
		n.UpdateNodeWatch <- true
	}
	n.NewNodeWatch = nil
	if n.ThreadSub != nil {
		go func() { _, _ = <-n.ThreadSub }()
		time.Sleep(5 * time.Millisecond)
		close(n.ThreadSub)
	}
	n.ThreadSub = nil
	if n.ThreadPub != nil {
		go func() { _, _ = <-n.ThreadPub }()
		time.Sleep(5 * time.Millisecond)
		close(n.ThreadPub)
	}
	n.ThreadPub = nil
	if n.PostPub != nil {
		go func() { _, _ = <-n.PostPub }()
		time.Sleep(5 * time.Millisecond)
		close(n.PostPub)
	}
	n.PostPub = nil
	//n.ThreadSubSocket.Close()
	//n.ThreadPubSocket.Close()
	//n.PostPubSocket.Close()
}

func NewNode(flags *FlagConfig) *Node {
	n := new(Node)
	n.Stats = NewNodeStats()
	n.Config.Hostname = flags.Hostname
	n.Config.BindIp = flags.BindIp
	n.Config.Etcd = strings.Split(flags.Etcd, ",")
	n.Config.Cluster = flags.ClusterName
	if flags.OnlyBoards != "" {
		n.Config.OnlyBoards = strings.Split(flags.OnlyBoards, ",")
	}
	if flags.ExcludeBoards != "" {
		n.Config.ExcludeBoards = strings.Split(flags.ExcludeBoards, ",")
	}
	n.Shutdown = false
	n.Closed = false
	return n
}

func (n *Node) Bootstrap() error {
	log.Print("Bootstrapping node...")
	n.NodeId = randNodeId()
	log.Print("Node Id:", n.NodeId)

	log.Print("Downloading 4Chan boards list...")
	var err error
	n.Boards, err = fourchan.DownloadBoards(n.Config.OnlyBoards, n.Config.ExcludeBoards)
	if err != nil {
		log.Print("Failed to download boards list: ", err)
		return err
	}

	if n.Config.Hostname == "" {
		n.Config.Hostname, _ = os.Hostname()
	}

	log.Print("Initializing publisher sockets...")
	tpubport := randInt(THRD_PUB_PORT_LOW, THRD_PUB_PORT_HIGH)
	ppubport := randInt(POST_PUB_PORT_LOW, POST_PUB_PORT_HIGH)
	if n.ThreadPubSocket, err = zmq3.NewSocket(zmq3.PUB); err != nil {
		log.Print("Failed to create Thread Pub Socket: ", err)
		return err
	}
	if n.PostPubSocket, err = zmq3.NewSocket(zmq3.PUB); err != nil {
		log.Print("Failed to create Post Pub Socket: ", err)
		return err
	}
	n.SocketWaitGroup.Add(2)

	if err := n.ThreadPubSocket.Bind(fmt.Sprintf("tcp://%s:%d", n.Config.BindIp, tpubport)); err != nil {
		log.Print((fmt.Sprintf("tcp://%s:%d", n.Config.BindIp, tpubport)))
		log.Print("Failed to bind Thread Pub Socket: ", err)
		return err
	}
	if err := n.PostPubSocket.Bind(fmt.Sprintf("tcp://%s:%d", n.Config.BindIp, ppubport)); err != nil {
		log.Print("Failed to bind Post Sub Socket: ", err)
		return err
	}

	n.EtcCluster = etcd.NewClient()
	if n.Config.Etcd != nil {
		if !n.EtcCluster.SetCluster(n.Config.Etcd) {
			log.Print("Failed to register to etcd.")
			return errors.New("ailed to register to etcd")
		}
	}

	for tries := 0; tries < 10; tries++ {
		log.Print("Registering to etcd...")
		resp, err := n.EtcCluster.Get(n.Config.Cluster + "/nodes")
		if err == nil || (isEtcdError(err) && (err.(etcd.EtcdError)).ErrorCode == 100) {
			var nodes []NodeInfo
			nodeIndex := 1
			if err == nil {
				result := resp[0]
				if e := json.Unmarshal([]byte(result.Value), &nodes); e != nil {
					log.Print("Failed to unmarshhal " + n.Config.Cluster + "/nodes")
					return e
				}
				for ; nodeIndex <= len(nodes); nodeIndex++ {
					found := false
					for _, node := range nodes {
						if nodeIndex == node.NodeIndex {
							found = true
							break
						}
					}
					if !found {
						break
					}
				}
			} else {
				nodes = make([]NodeInfo, 0, 1)
			}
			nodes = append(nodes, NodeInfo{n.NodeId, nodeIndex, n.Config.Hostname, tpubport, ppubport})
			if n.ThreadSubSocket, err = zmq3.NewSocket(zmq3.SUB); err != nil {
				log.Print("Failed to create Thread Sub Socket: ", err)
				return err
			}
			n.ThreadSubSocket.SetSubscribe("")
			n.SocketWaitGroup.Add(1)
			var failureErr error
			failureErr = nil
			for _, node := range nodes {
				log.Print("Connecting to ", fmt.Sprintf("tcp://%s:%d", node.Hostname, node.TPubPort))
				if err := n.ThreadSubSocket.Connect(fmt.Sprintf("tcp://%s:%d", node.Hostname, node.TPubPort)); err != nil {
					log.Print(err)
					failureErr = err
					break
				}
			}
			if failureErr != nil {
				n.Close()
				log.Print("Failed to create Thread Sub Socket: ", err)
				time.Sleep(2 * time.Second)
				continue
			}
			log.Print("Finished connecting, watching nodes...")
			n.NewNodeWatch = make(chan bool)
			n.UpdateNodeWatch = make(chan bool)
			newNodes := make(chan *store.Response)
			updateNodes := make(chan *store.Response)

			n.ThreadPub = make(chan fourchan.ThreadInfo)
			n.PostPub = make(chan fourchan.Post)
			n.ThreadSub = make(chan fourchan.ThreadInfo)

			go n.bootstrapThreadWorkers()

			go func() {
				for !n.Closed {
					data, err := n.ThreadSubSocket.RecvBytes(zmq3.DONTWAIT)
					if err == syscall.EAGAIN {
						time.Sleep(1 * time.Millisecond)
						continue
					}
					var threadInfo fourchan.ThreadInfo
					dec := gob.NewDecoder(bytes.NewBuffer(data))
					err = dec.Decode(&threadInfo)
					if err != nil {
						log.Print("Failed to decode thread gob ", err)
						continue
					}
					if n.ThreadSub != nil {
						n.ThreadSub <- threadInfo
					}
				}
				n.ThreadSubSocket.Close()
				n.SocketWaitGroup.Done()
			}()

			go func(newNodes <-chan *store.Response) {
				for response := range newNodes {
					if response.Action == "SET" {
						if fmt.Sprintf("%s:%d", n.Config.Hostname, tpubport) != response.Value {
							e := n.ThreadSubSocket.Connect(fmt.Sprintf("tcp://%s", response.Value))
							if e != nil {
								log.Print("Failed to connect to", fmt.Sprintf("tcp://%s", response.Value), ":", e)
							}
						}
					}
				}
			}(newNodes)

			go func(newNodes chan *store.Response) {
				for {
					_, e := n.EtcCluster.Watch(n.Config.Cluster+"/add-node", 0, newNodes, n.NewNodeWatch)
					if e.Error() == "User stoped watch" {
						break
					}
				}
				close(newNodes)
			}(newNodes)

			go func(updateNodes chan *store.Response) {
				for response := range updateNodes {
					if response.Action == "SET" && response.Key == "/"+n.Config.Cluster+"/nodes" {
						n.divideBoards()
					}
				}
			}(updateNodes)

			go func(updateNodes chan *store.Response) {
				for {
					_, e := n.EtcCluster.Watch(n.Config.Cluster+"/nodes", 0, updateNodes, n.UpdateNodeWatch)
					if e.Error() == "User stoped watch" {
						break
					}
				}
				close(updateNodes)
			}(updateNodes)

			go func() {
				for threadInfo := range n.ThreadPub {
					var buff bytes.Buffer
					enc := gob.NewEncoder(&buff)
					err := enc.Encode(threadInfo)
					if err != nil {
						log.Print("Failed to encode threadInfo ", err)
						continue
					}
					for tries := 0; tries < 16; tries++ {
						_, e := n.ThreadPubSocket.SendBytes(buff.Bytes(), zmq3.DONTWAIT)
						if e == nil {
							break
						}
					}
				}
				n.ThreadPubSocket.Close()
				n.SocketWaitGroup.Done()
			}()

			go func() {
				for post := range n.PostPub {
					var buff bytes.Buffer
					enc := gob.NewEncoder(&buff)
					err := enc.Encode(post)
					if err != nil {
						log.Print("Failed to encode post ", err)
						continue
					}
					for tries := 0; tries < 16; tries++ {
						_, e := n.PostPubSocket.SendBytes(buff.Bytes(), zmq3.DONTWAIT)
						if e == nil {
							break
						}
					}
				}
				n.PostPubSocket.Close()
				n.SocketWaitGroup.Done()
			}()

			sort.Sort(NodeInfoList(nodes))
			for idx, _ := range nodes {
				nodes[idx].NodeIndex = idx + 1
			}

			newNodeData, _ := json.Marshal(nodes)
			prevValue := ""
			if len(resp) > 0 {
				prevValue = resp[0].Value
			}

			time.Sleep(1 * time.Second)
			log.Print("Updating node list.")

			_, isSet, err := n.EtcCluster.TestAndSet(n.Config.Cluster+"/nodes", prevValue, string(newNodeData), 0)
			if err != nil || isSet != true {
				log.Print("Failed to update node list. Possibly another node bootstrapped before finish.")
				n.Close()
				continue
			}
			//log.Print("SETTING")
			n.EtcCluster.Set(n.Config.Cluster+"/add-node/"+n.NodeId, fmt.Sprintf("%s:%d", n.Config.Hostname, tpubport), 600)
			n.EtcCluster.Set(n.Config.Cluster+"/add-postpub/"+n.NodeId, fmt.Sprintf("%s:%d", n.Config.Hostname, ppubport), 600)
			log.Print("Complete bootstrapping node.")
			return nil
		} else {
			log.Print("Failed to register to etcd: ", err)
			return err
		}
	}

	log.Print("Failed to bootstrap node.")
	return errors.New("Failed to bootstrap node.")
}

func (n *Node) divideBoards() {
	n.DivideMutex.Lock()
	defer n.DivideMutex.Unlock()
	if n.BoardStop != nil {
		for _, c := range n.BoardStop {
			c <- true
		}
	}
	resp, err := n.EtcCluster.Get(n.Config.Cluster + "/nodes")
	if err == nil {
		var nodes []NodeInfo
		result := resp[0]
		if e := json.Unmarshal([]byte(result.Value), &nodes); e != nil {
			log.Print("Failed to unmarshhal " + n.Config.Cluster + "/nodes")
			return
		}
		sort.Sort(NodeInfoList(nodes))
		nodeIds := make([]string, 0, 16)
		for idx, node := range nodes {
			nodes[idx].NodeIndex = idx + 1
			nodeIds = append(nodeIds, node.NodeId)
		}
		newNodeData, _ := json.Marshal(nodes)
		if string(newNodeData) != resp[0].Value {
			_, _, _ = n.EtcCluster.TestAndSet(n.Config.Cluster+"/nodes", resp[0].Value, string(newNodeData), 0)
			return
		}
		var myNode *NodeInfo
		for idx, node := range nodes {
			if node.NodeId == n.NodeId {
				myNode = &nodes[idx]
			}
		}
		if myNode == nil {
			if n.Shutdown == false {
				log.Print("Oh dear, I can't find myself in the config, I may be dead.")
				go n.CleanShutdown()
			}
			return
		}
		n.BoardStop = make([]chan bool, 0, 64)
		n.OwnedBoards = make([]string, 0, 64)
		n.LastNodeIdx = myNode.NodeIndex
		n.NodeCount = len(nodes)
		for idx, board := range n.Boards.Boards {
			if len(nodes) != 0 && (idx%len(nodes))+1 == myNode.NodeIndex {
				bc := make(chan bool)
				n.BoardStop = append(n.BoardStop, bc)
				n.OwnedBoards = append(n.OwnedBoards, board.Board)
				go n.ProcessBoard(nodeIds, board.Board, bc)
			}
		}
	} else {
		log.Print("Failed to get node configuration.")
		return
	}
}

func (n *Node) bootstrapThreadWorkers() {
	var wg sync.WaitGroup
	for i := 0; i < THREAD_WORKERS; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ti := range n.ThreadSub {
				if ti.OwnerId == n.NodeId {
					n.ProcessThread(ti)
				}
			}
		}()
	}

	wg.Wait()
}

func (n *Node) CleanShutdown() {
	if n.Shutdown {
		return
	}
	n.Shutdown = true
	log.Print("Removing node from cluster.")
	for tries := 0; tries < 32; tries++ {
		resp, err := n.EtcCluster.Get(n.Config.Cluster + "/nodes")
		if err == nil {
			var nodes []NodeInfo
			result := resp[0]
			if e := json.Unmarshal([]byte(result.Value), &nodes); e != nil {
				log.Print("Failed to unmarshhal " + n.Config.Cluster + "/nodes")
				return
			}
			sort.Sort(NodeInfoList(nodes))
			for idx, _ := range nodes {
				nodes[idx].NodeIndex = idx + 1
			}

			newNodes := make([]NodeInfo, 0, 16)
			for _, node := range nodes {
				if node.NodeId != n.NodeId {
					newNodes = append(newNodes, node)
				}
			}
			newNodeData, _ := json.Marshal(newNodes)
			_, isSet, err := n.EtcCluster.TestAndSet(n.Config.Cluster+"/nodes", resp[0].Value, string(newNodeData), 0)
			if err != nil || isSet != true {
				log.Print("Failed to update node list. Possibly another node bootstrapped before finish.")
				continue
			}
			n.EtcCluster.Set(n.Config.Cluster+"/nodes/remove/"+n.NodeId, fmt.Sprintf("%s", n.NodeId), 60)
			break
		}
	}
	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(120 * time.Second)
		if false {
			log.Print("Timeout waiting for sockets.")
			stack := make([]byte, 262144)
			runtime.Stack(stack, true)
			log.Print("----------- DUMP STACK CALLED ----------------")
			log.Print("\n", string(stack))
		}
		timeout <- true
	}()
	go func() {
		log.Print("Closing sockets...")
		n.Close()
		n.SocketWaitGroup.Wait()
		log.Print("Cleaning up...")
		n.RoutineWaitGroup.Wait()
		log.Print("Shut down node.")
		timeout <- true
	}()
	<-timeout
}
