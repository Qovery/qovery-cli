package pkg

import (
	"errors"
	"fmt"
	"github.com/appscode/go-querystring/query"
	"github.com/gorilla/websocket"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
)

type PortForwardRequest struct {
	ServiceID      utils.Id `url:"service"`
	EnvironmentID  utils.Id `url:"environment"`
	ProjectID      utils.Id `url:"project"`
	OrganizationID utils.Id `url:"organization"`
	ClusterID      utils.Id `url:"cluster"`
	PodName        string   `url:"pod_name,omitempty"`
	ServiceType    string   `url:"service_type"`
	Port           uint16   `url:"port"`
	LocalPort      uint16
}

type WebsocketPortForward struct {
	ws *websocket.Conn
}

func (w WebsocketPortForward) Write(p []byte) (n int, err error) {
	err = w.ws.WriteMessage(websocket.BinaryMessage, p)

	return len(p), err
}
func (w WebsocketPortForward) Read(p []byte) (n int, err error) {
	for {
		msgType, msg, err := w.ws.ReadMessage()
		if err != nil {
			return 0, err
		}

		if msgType == websocket.CloseMessage {
			return 0, io.EOF
		}

		if msgType != websocket.BinaryMessage {
			continue
		}

		return copy(p, msg), err
	}
}

func mkWebsocketConn(req *PortForwardRequest) (*WebsocketPortForward, error) {
	command, err := query.Values(req)
	if err != nil {
		return nil, err
	}

	wsURL, err := url.Parse("wss://ws.qovery.com/shell/portforward")
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

	ws := WebsocketPortForward{ws: wsConn}
	return &ws, nil
}

func ExecPortForward(req *PortForwardRequest) {
	listen, error := net.Listen("tcp", fmt.Sprintf("localhost:%d", req.LocalPort))

	// Handles eventual errors
	if error != nil {
		fmt.Println(error)
		return
	}

	fmt.Printf("Listening on %s => %d\n", listen.Addr().String(), req.Port)

	for {
		// Accepts connections
		con, error := listen.Accept()

		// Handles eventual errors
		if error != nil {
			fmt.Println(error)
			continue
		}

		go handleConnection(con, req)
	}
}

func handleConnection(con net.Conn, req *PortForwardRequest) {
	var errRet error
	fmt.Printf("Connection accepted from %s => %d\n", con.RemoteAddr().String(), req.Port)
	defer func() {
		con.Close()
		fmt.Printf("Connection closed from %s => %d\n", con.RemoteAddr().String(), req.Port)
		var e *websocket.CloseError
		if errors.As(errRet, &e) && e.Code != websocket.CloseNormalClosure {
			log.Error("connection terminated badly with ", e)
		}
	}()

	wsConn, err := mkWebsocketConn(req)
	if err != nil {
		log.Fatal("error while creating websocket connection", err)
	}
	defer func() {
		wsConn.ws.Close()
	}()

	go func() {
		_, _ = io.Copy(wsConn, con)
	}()
	_, err = io.Copy(con, wsConn)
	errRet = err
}
