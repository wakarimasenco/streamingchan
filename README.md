# STREAMINGCHAN
September 8th, 2013

[streaming.wakarimasen.co](http://streaming.wakarimasen.co/)

StreamingChan is a distributed api designed to create a "real-time" like endpoint for the [4Chan API](https://github.com/4chan/4chan-API). This is mostly an experiment to see if there any interesting applications of having 4Chan posts as they come in. Currently, there is an endpoint hosted at [streaming.wakarimasen.co](http://streaming.wakarimasen.co/) and can be used following the docs below.

## Installation
The infrastructure contains 3 parts. A node, which is responsible for downloading data from 4chan. An api endpoint, which is resposible for streaming data to a user. And finally, [etcd](github.com/coreos/etcd) which is resposible for discovery between nodes (not unlike Apache ZooKeeper).

### Dependencies

+ [Go 1.1](http://golang.org)
+ [etcd](github.com/coreos/etcd)
+ [ØMQ 3.2](http://zeromq.org/) (`brew install zeromq` on OS X with brew)
+ [zmq3](https://github.com/pebbe/zmq3) (Go Bindings `go get github.com/pebbe/zmq3`)
+ [go-etcd](https://github.com/coreos/go-etcd) (`go get github.com/coreos/go-etcd/etcd`)

### Building
If you have the appropriate dependecies, simply build with `go build`. Alternatively you can run `./version.sh` to embed the build date and git hash into the build.

### Running
First, ensure you have atleast one `etcd` instance running somewhere.

#### Starting a Node
Nodes are responsible for download data from the 4Chan Api. To run a node, run `streamingchan -node`
##### Command Line Options

+ `etcd` - Etcd Machines - *Required:* Comma seperated list of etcd machines. Ex. `-etcd="http://127.0.0.1:4001,http://192.168.1.3:4001"`
+ `tw` - Threads - Number of thread downloaders running concurrently. Default : 10
+ `hostname` - Hostname - Hostname or IP of this machine that will be visible to other machines on the network. Default : OS default
+ `bindip` - Bind IP - Interface to bind to. Use 0.0.0.0 to bind to all. Default : `127.0.0.1`
+ `clustername` - Cluster Name - Run multiple clusters on the same etcd cluster with this option. Default : streamingchan
+ `onlyboards` - Only Boards - Only download from this comma seperated list of boards. If empty, download from all boards.
+ `excludeboards` - Exclude Boards - Exclude these boards from being downloads. Ex. `-excludeboards=b,sp,v`
+ `httpport` - Http Port - Each node will expose a `/status/` and `/commands/` api.

#### Node Http Server
The Node's http server is useful for looking at stats or sending commands.
##### Endpoints

+ `/status/` - Returns a JSON response with various stats.
+ `/commands/` - Executes commands on the node server.
+ `/commands/stop` - Stops the server

#### Example cmdline
`streamingchan -node -etcd="http://127.0.0.1:4001" -onlyboards="b" -hostname="127.0.0.1"`

#### Starting an API Server
API servers are the endpoints that stream all the post data.
##### Command Line Options

+ `etcd` - Etcd Machines - *Required:* Comma seperated list of etcd machines. Ex. `-etcd="http://127.0.0.1:4001,http://192.168.1.3:4001"`
+ `bindip` - Bind IP (Ignored) - Interface to bind to. Use 0.0.0.0 to bind to all. Default : `127.0.0.1`
+ `clustername` - Cluster Name - Run multiple clusters on the same etcd cluster with this option. Default : streamingchan
+ `httpport` - Http Port - Port to listen on

#### Example cmdline
`streamingchan -api -etcd="http://127.0.0.1:4001"`

## API
Currently `/stream.json` is accessible from [streaming.wakarimasen.co](http://streaming.wakarimasen.co/).

### GET /stream.json
#### Resource URL
`http://streaming.wakarimasen.co/stream.json`
#### Parameters
##### Filter

+ `board` - String, Board Id - Filter for only posts from this board.
+ `thread` - String, "boardId/resto" - Filter for only posts from this thread.
+ `comment` - String - Filter for posts that only contain this comment.
+ `name` - String - Filter for posts that only match this name (Substring match).
+ `trip` - String - Filter for posts that only match this trip (Substring match).
+ `email` - String - Filter for posts that only match this email (Substring match).

Filters can be chained, and a post must match *all* the filters. So `/stream.json?name=Diaz&board=a&board=v&comment=hi10p` will only match posts with name Diaz, on the /a/ and /v/ boards, that has "hi10p" in the comment.

Threads must be denoted like this "boardId/resto" (Ex. `/stream.json?thread=sp/2302394`)

#### Returns
Returns and unbounded list of JSON post objects as defined by the [4Chan API](https://github.com/4chan/4chan-API#posts-object), with one exception. A field `board` has been added to denote the board the post originated from.

Each json object will be seperated by a carriage return (`\r\n`). Any new lines in the post will be denoted with a regular new line (`\n`).

If the API hasn't sent a message in atleast 30 seconds, the API will send a blank line (`\r\n`). If nothing has ever been sent, the API will send an empty JSON object, followed by a carriage return (`{}\r\n`).

## TODO

+ Purge Dead Nodes