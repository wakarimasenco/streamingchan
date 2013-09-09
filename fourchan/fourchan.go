package fourchan

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"time"
)

var DefaultClient *http.Client

const (
	USER_AGENT = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_2) AppleWebKit/537.11 (KHTML, like Gecko) Chrome/23.0.1271.101 Safari/537.11"
)

func init() {
	DefaultClient = &http.Client{}
}

func EasyGet(url string, lastModified time.Time) ([]byte, string, int, string, error) {

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("User-Agent", USER_AGENT)
	if !lastModified.IsZero() {
		req.Header.Add("If-Modified-Since", lastModified.Format(http.TimeFormat))
	}

	resp, net_error := DefaultClient.Do(req)

	if resp == nil {
		// You can get an error (such as 404), but still recieve a response
		// In this case you don't get a response (such as connection timeout)
		return nil, "", 0, "", net_error
	}
	defer resp.Body.Close()

	data, read_error := ioutil.ReadAll(resp.Body)
	if net_error != nil {
		return data, resp.Header.Get("Content-Type"), resp.StatusCode, resp.Header.Get("Last-Modified"), net_error
	} else {
		return data, resp.Header.Get("Content-Type"), resp.StatusCode, resp.Header.Get("Last-Modified"), read_error
	}
	panic("Unreachable")
}

func contains(a []string, x string) bool {
	i := sort.SearchStrings(a, x)
	return !(i == len(a) || a[i] != x)
}

func DownloadBoards(only []string, exclude []string) (*Boards, error) {
	if data, _, _, _, err := EasyGet("http://api.4chan.org/boards.json", time.Time{}); err == nil {
		b := new(Boards)
		if err := json.Unmarshal(data, &b); err == nil {
			if (only == nil || len(only) == 0) && (exclude == nil || len(exclude) == 0) {
				return b, nil
			}
			filteredBoards := new(Boards)
			filteredBoards.Boards = make([]Board, 0, 64)
			sort.Strings(only)
			sort.Strings(exclude)
			for _, board := range b.Boards {
				if exclude != nil && len(exclude) != 0 {
					if contains(exclude, board.Board) {
						continue
					}
				}
				if only != nil && len(only) != 0 {
					if contains(only, board.Board) {
						filteredBoards.Boards = append(filteredBoards.Boards, board)
					}
				} else {
					filteredBoards.Boards = append(filteredBoards.Boards, board)
				}
			}
			return filteredBoards, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func DownloadBoard(board string, lastModified time.Time) ([]Threads, int, string, error) {
	if data, _, statusCode, lastModified, err := EasyGet("http://api.4chan.org/"+board+"/threads.json", lastModified); err == nil {
		var t []Threads
		if err := json.Unmarshal(data, &t); err == nil {
			for idx, _ := range t {
				t[idx].Board = board
			}
			return t, statusCode, lastModified, nil
		} else {
			return nil, statusCode, lastModified, err
		}
	} else {
		return nil, 500, "", err
	}
}

func DownloadThread(board string, thread int) (Thread, error) {
	if data, _, _, _, err := EasyGet(fmt.Sprintf("http://api.4chan.org/%s/res/%d.json", board, thread), time.Time{}); err == nil {
		var t Thread
		if err := json.Unmarshal(data, &t); err == nil {
			for idx, _ := range t.Posts {
				t.Posts[idx].Board = board
			}
			return t, nil
		} else {
			return t, err
		}
	} else {
		return Thread{}, err
	}
}
