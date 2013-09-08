package node

import (
	"flag"
	"os"
)

var Flags *FlagConfig

type FlagConfig struct {
	ThreadWorkers int
	Hostname      string
	BindIp        string
	Etcd          string
	ClusterName   string
	OnlyBoards    string
	ExcludeBoards string
	HttpPort      int
	Node          bool
}

func init() {
	for _, arg := range os.Args {
		if arg == "-node" {
			Flags = flags()
		}
	}
}

func flags() *FlagConfig {
	fc := new(FlagConfig)
	hostname, _ := os.Hostname()
	flag.IntVar(&(fc.ThreadWorkers), "tw", THREAD_WORKERS, "Node : Number of concurrent thread downloaders.")
	flag.BoolVar(&(fc.Node), "node", false, "Node : Enable node proces. ")
	flag.StringVar(&(fc.Hostname), "hostname", hostname, "Node : Hostname or ip, visible from other machines on the network. ")
	flag.StringVar(&(fc.BindIp), "bindip", "127.0.0.1", "Node : Address to bind to.")
	flag.StringVar(&(fc.Etcd), "etcd", "", "Node : Etcd addresses (Comma seperated)")
	flag.StringVar(&(fc.ClusterName), "clustername", "streamingchan", "Node : Cluster name")
	flag.StringVar(&(fc.OnlyBoards), "onlyboards", "", "Node : Boards (a,b,sp) to process. Comma seperated.")
	flag.StringVar(&(fc.ExcludeBoards), "excludeboards", "", "Node : Boards (a,b,sp) to exclude. Comma seperated.")
	flag.IntVar(&(fc.HttpPort), "httpport", PORT_NUMBER, "Node : Host for HTTP Server for serving stats. 0 for disabled.")

	return fc
}
