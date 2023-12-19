package pkg

import (
	"github.com/appscode/go-querystring/query"
	"net/http"
	"net/url"
	"regexp"

	"github.com/containerd/console"
	"github.com/gorilla/websocket"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
)

const StdinBufferSize = 4096

type ShellRequest struct {
	ServiceID      utils.Id `url:"service"`
	EnvironmentID  utils.Id `url:"environment"`
	ProjectID      utils.Id `url:"project"`
	OrganizationID utils.Id `url:"organization"`
	ClusterID      utils.Id `url:"cluster"`
	PodName        *string  `url:"pod_name,omitempty"`
	ContainerName  *string  `url:"container_name,omitempty"`
	Command        []string `url:"command"`
}

func ExecShell(req *ShellRequest) {
	wsConn, err := createWebsocketConn(req)
	if err != nil {
		log.Fatal("error while creating websocket connection", err)
	}
	defer func() {
		if err := wsConn.Close(); err != nil {
			log.Fatal("error while closing websocket connection", err)
		}
	}()

	currentConsole := console.Current()
	if err := currentConsole.SetRaw(); err != nil {
		log.Fatal("error while setting up console", err)
	}

	done := make(chan struct{})
	stdIn := make(chan []byte)

	go readWebsocketConnection(wsConn, currentConsole, done)
	go readUserConsole(currentConsole, stdIn, done)

	for {
		select {
		case <-done:
			return
		case msg := <-stdIn:
			if err := wsConn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				log.Error("error while writing on websocket:", err)
				return
			}
		}
	}
}

func createWebsocketConn(req *ShellRequest) (*websocket.Conn, error) {
	command, err := query.Values(req)
	if err != nil {
		return nil, err
	}

	wsURL, err := url.Parse("wss://ws.qovery.com/shell/exec")
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
	return wsConn, nil
}

func readWebsocketConnection(wsConn *websocket.Conn, currentConsole console.Console, done chan struct{}) {
	defer close(done)
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
		if _, err = currentConsole.Write(msg); err != nil {
			log.Error("error while writing in console:", err)
			return
		}
	}
}

func readUserConsole(currentConsole console.Console, stdIn chan []byte, done chan struct{}) {
	defer close(done)
	buffer := make([]byte, StdinBufferSize)
	for {
		count, err := currentConsole.Read(buffer)
		if err != nil {
			log.Error("error while reading on console:", err)
			return
		}
		stdIn <- buffer[0:count]
	}
}
