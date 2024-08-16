package pkg

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"time"
)

type LogRequest struct {
	ServiceID      utils.Id
	EnvironmentID  utils.Id
	ProjectID      utils.Id
	OrganizationID utils.Id
	ClusterID      utils.Id
	RawFormat      bool
}

type LogMessage struct {
	CreatedAt Timestamp `json:"created_at"`
	Message   string    `json:"message"`
	Version   string    `json:"version"`
	PodName   string    `json:"pod_name"`
}

func ExecLog(req *LogRequest) {
	wsConn, err := createLogWebsocket(req)
	if err != nil {
		log.Fatal("error while creating websocket connection", err)
	}
	defer func() {
		if err := wsConn.Close(); err != nil {
			log.Fatal("error while closing websocket connection", err)
		}
	}()

	var logMessage LogMessage
	for {
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			if e, ok := err.(*websocket.CloseError); ok {
				log.Error("connection closed by server: ", e)
				return
			}
			log.Error("error while reading on websocket:", err)
			return
		}

		if req.RawFormat {
			fmt.Printf("%s\n", msg)
		} else {
			err = json.Unmarshal(msg, &logMessage)
			if err != nil {
				log.Fatal("%", err)
			}
			fmt.Printf("| %s | %s | %s\n", logMessage.CreatedAt.Format("2006-01-02 15:04:05.000"), logMessage.PodName, logMessage.Message)
		}
	}
}

func createLogWebsocket(req *LogRequest) (*websocket.Conn, error) {
	wsURL, err := url.Parse(fmt.Sprintf(
		"%s/service/logs?service=%s&cluster=%s&environment=%s&organization=%s&project=%s",
		utils.WebsocketUrl(),
		req.ServiceID,
		req.ClusterID,
		req.EnvironmentID,
		req.OrganizationID,
		req.ProjectID,
	))
	if err != nil {
		return nil, err
	}

	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		return nil, err
	}

	headers := http.Header{"Authorization": {utils.GetAuthorizationHeaderValue(tokenType, token)}}
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), headers)
	if err != nil {
		return nil, err
	}
	return wsConn, nil
}

type Timestamp struct {
	time.Time
}

// UnmarshalJSON decodes an int64 timestamp into a time.Time object
func (p *Timestamp) UnmarshalJSON(bytes []byte) error {
	// 1. Decode the bytes into an int64
	var raw int64
	err := json.Unmarshal(bytes, &raw)

	if err != nil {
		fmt.Printf("error decoding timestamp: %s\n", err)
		return err
	}

	// 2. Parse the unix timestamp
	p.Time = time.UnixMilli(raw)
	return nil
}
