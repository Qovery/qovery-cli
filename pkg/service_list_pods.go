package pkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/appscode/go-querystring/query"
	"github.com/gorilla/websocket"
	"github.com/qovery/qovery-cli/utils"
	"net/http"
	"net/url"
	"regexp"
)

type PodResponse struct {
	Name  string
	Ports []uint16
}
type ListPodResponse struct {
	Pods []PodResponse
}

func ExecListPods(req *PortForwardRequest) (*ListPodResponse, error) {
	command, err := query.Values(req)
	if err != nil {
		return nil, err
	}

	wsURL, err := url.Parse(fmt.Sprintf("%s/service/pods", utils.WebsocketUrl()))
	if err != nil {
		return nil, err
	}
	pattern := regexp.MustCompile("%5B([0-9]+)%5D=")
	wsURL.RawQuery = pattern.ReplaceAllString(command.Encode(), "[${1}]=")

	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		return nil, err
	}

	headers := http.Header{"Authorization": {utils.GetAuthorizationHeaderValue(tokenType, token)}}
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), headers)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = wsConn.Close()
	}()

	msgType, payload, err := wsConn.ReadMessage()
	if err != nil {
		return nil, err
	}

	switch msgType {
	case websocket.TextMessage:
		var data ListPodResponse
		err = json.Unmarshal(payload, &data)
		if err != nil {
			return nil, err
		}
		return &data, nil
	default:
		return nil, errors.New("received invalid message while listing pods: " + string(rune(msgType)) + " " + string(payload))
	}
}
