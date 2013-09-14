package client

import (
	"encoding/json"
	"fmt"
	"github.com/wakarimasenco/streamingchan/fourchan"
	"io"
	"net"
	"net/http"
	"time"
)

type Client struct {
	StreamUri string

	decoder    *json.Decoder
	connection net.Conn
	client     *http.Client
	closer     io.Closer
	stream     chan fourchan.Post
	running    bool
}

func (c *Client) readLoop() {
	for c.running {
		var post fourchan.Post
		c.connection.SetReadDeadline(time.Now().Add(3 * time.Second))
		if err := c.decoder.Decode(&post); err != nil {
			continue
		}
		c.stream <- post
	}
	close(c.stream)
}

func (c *Client) GetStream() <-chan fourchan.Post {
	if c.stream == nil {
		panic("Attempted to get stream before stream has started.")
	}
	return c.stream
}

func (c *Client) NextPost() fourchan.Post {
	if c.stream == nil {
		panic("Attempted to get stream before stream has started.")
	}
	return <-c.stream
}

func (c *Client) Start() error {
	req, _ := http.NewRequest("GET", c.StreamUri, nil)

	resp, err := c.client.Do(req)
	if resp.StatusCode != 200 || err != nil {
		return fmt.Errorf("Failed to connect to stream \"%s\"", c.StreamUri)
	}
	c.closer = resp.Body
	c.decoder = json.NewDecoder(resp.Body)
	c.running = true
	c.stream = make(chan fourchan.Post, 8)
	go c.readLoop()
	return nil
}

func (c *Client) Stop() error {
	c.running = false
	for _ = range c.stream {
	}
	c.stream = nil
	// Have to close the raw connection, since closing the response body reader
	// will make Go try to read the request, which goes on forever.
	if err := c.connection.Close(); err != nil {
		c.closer.Close()
		return err
	}
	return c.closer.Close()
}

func NewClient() *Client {
	c := new(Client)
	c.stream = nil
	c.StreamUri = "http://streaming.wakarimasen.co/stream.json"
	dialer := func(netw, addr string) (net.Conn, error) {
		netc, err := net.DialTimeout(netw, addr, 3*time.Second)
		if err != nil {
			return nil, err
		}
		c.connection = netc
		return netc, nil
	}

	c.client = &http.Client{
		Transport: &http.Transport{
			Dial: dialer,
		},
	}

	return c
}
