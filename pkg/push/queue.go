package push

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/api"

	"github.com/flanksource/commons/logger"
	goqueue "github.com/phf/go-queue/queue"
)

var Queues = make(map[string]*goqueue.Queue)

func AddServers(servers []string) {
	for _, name := range servers {
		Queues[name] = goqueue.New()
	}
}

func Queue(check pkg.Check, status pkg.CheckStatus) {
	data := api.QueueData{
		Check:  check,
		Status: status,
	}
	for _, queue := range Queues {
		queue.PushBack(data)
	}
}

func Start() {
	for server, queue := range Queues {
		go consumeQueue(server, queue)
	}
}

func consumeQueue(server string, queue *goqueue.Queue) {
	for {
		element := queue.PopBack()
		if element == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		data := element.(api.QueueData)
		jsonData, err := json.Marshal(data)
		if err != nil {
			logger.Errorf("error unmarshalling request body: %v", err)
			continue
		}
		err = api.PostDataToServer(strings.TrimSpace(server), bytes.NewBuffer(jsonData))
		if err != nil {
			logger.Errorf("error sending data to server %v body: %v", server, err)
			continue
		}
	}
}
