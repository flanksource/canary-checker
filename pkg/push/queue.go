package push

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	goqueue "github.com/phf/go-queue/queue"
)

var Queues = make(map[string]*goqueue.Queue)

func AddServers(servers []string) {
	for _, name := range servers {
		Queues[name] = goqueue.New()
	}
}

func Queue(check pkg.Check) {
	for _, queue := range Queues {
		queue.PushBack(check)
	}
}

func Start() {
	for {
		for server, queue := range Queues {
			go consumeQueue(server, queue)
		}
		time.Sleep(10 * time.Second)
	}
}

func consumeQueue(server string, queue *goqueue.Queue) {
	for queue.Len() != 0 {
		check := queue.PopBack().(pkg.Check)
		jsonData, err := json.Marshal(check)
		if err != nil {
			logger.Errorf("error unmarshalling request body: %v", err)
			return
		}
		err = PostDataToServer(strings.TrimSpace(server), bytes.NewBuffer(jsonData))
		if err != nil {
			logger.Errorf("error sending data to server %v body: %v", server, err)
			return
		}
	}
}
