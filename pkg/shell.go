package pkg

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/appscode/go-querystring/query"
	"github.com/containerd/console"
	"github.com/gorilla/websocket"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
)

const StdinBufferSize = 4096
const ReconnectDelay = 5 * time.Second
const PingInterval = 30 * time.Second
const ReadTimeout = 60 * time.Second

type TerminalSize interface {
	SetTtySize(width uint16, height uint16)
}

type ShellRequest struct {
	ServiceID      utils.Id `url:"service"`
	EnvironmentID  utils.Id `url:"environment"`
	ProjectID      utils.Id `url:"project"`
	OrganizationID utils.Id `url:"organization"`
	ClusterID      utils.Id `url:"cluster"`
	PodName        string   `url:"pod_name,omitempty"`
	ContainerName  string   `url:"container_name,omitempty"`
	Command        []string `url:"command"`
	TtyWidth       uint16   `url:"tty_width"`
	TtyHeight      uint16   `url:"tty_height"`
}

func (s *ShellRequest) SetTtySize(width uint16, height uint16) {
	s.TtyWidth = width
	s.TtyHeight = height
}

func ExecShell(req TerminalSize, path string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	var userCancelled atomic.Bool
	var normalExit atomic.Bool

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	go func() {
		<-signalChan
		userCancelled.Store(true)
		cancel()
	}()

	currentConsole := console.Current()
	defer func() {
		_ = currentConsole.Reset()
	}()

	if err := currentConsole.SetRaw(); err != nil {
		log.Fatal("error while setting up console", err)
	}

	winSize, err := currentConsole.Size()
	if err != nil {
		log.Fatal("Cannot get terminal size", err)
	}
	req.SetTtySize(winSize.Width, winSize.Height)

	stdIn := make(chan []byte)
	wg.Add(1)
	go readUserConsole(ctx, cancel, currentConsole, stdIn, &normalExit, &wg)

	for {
		if ctx.Err() != nil || userCancelled.Load() || normalExit.Load() {
			log.Info("Shell exited, not reconnecting.")
			break
		}

		log.Info("Attempting to (re)connect to WebSocket")

		wsConn, err := createWebsocketConn(req, path)
		if err != nil {
			log.Errorf("WebSocket connection failed: %v", err)
			if ctx.Err() != nil || userCancelled.Load() || normalExit.Load() {
				log.Info("User cancelled or shell exited during connection attempt.")
				break
			}
			time.Sleep(ReconnectDelay)
			continue
		}

		done := make(chan struct{})
		wg.Add(1)
		go readWebsocketConnection(ctx, wsConn, currentConsole, done, &normalExit, &wg)

		pingTicker := time.NewTicker(PingInterval)

	wsLoop:
		for {
			select {
			case <-ctx.Done():
				_ = wsConn.Close()
				break wsLoop
			case <-done:
				_ = wsConn.Close()
				break wsLoop
			case msg := <-stdIn:
				if err := wsConn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
					log.Error("Write error:", err)
					_ = wsConn.Close()
					break wsLoop
				}
			case <-pingTicker.C:
				if err := wsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Error("Ping error:", err)
					_ = wsConn.Close()
					break wsLoop
				}
			}

			if normalExit.Load() || userCancelled.Load() || ctx.Err() != nil {
				break wsLoop
			}
		}

		pingTicker.Stop()

		// Cancel the context to notify readUserConsole
		if normalExit.Load() || userCancelled.Load() {
			cancel()
		}

		// Do NOT close stdIn — readUserConsole owns it and it is used across reconnects.
		time.Sleep(ReconnectDelay)
	}

	wg.Wait()
}

func createWebsocketConn(req interface{}, path string) (*websocket.Conn, error) {
	command, err := query.Values(req)
	if err != nil {
		return nil, err
	}

	wsURL, err := url.Parse(fmt.Sprintf("%s%s", utils.WebsocketUrl(), path))
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
	conn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), headers)
	return conn, err
}

func readWebsocketConnection(ctx context.Context, wsConn *websocket.Conn, currentConsole console.Console, done chan struct{}, normalExit *atomic.Bool, wg *sync.WaitGroup) {
	defer wg.Done()

	var once sync.Once
	safeClose := func() {
		once.Do(func() {
			select {
			case <-done:
				// already closed
			default:
				close(done)
			}
		})
	}
	defer safeClose()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msgType, msg, err := wsConn.ReadMessage()
			if err != nil {
				var e *websocket.CloseError
				if errors.As(err, &e) {
					if e.Code == websocket.CloseNormalClosure {
						log.Info("** shell terminated bye **")
						normalExit.Store(true)
					} else {
						log.Errorf("connection closed by server: %v", e)
					}
					return
				}
				log.Errorf("error while reading on websocket: %v", err)
				return
			}

			if msgType == websocket.CloseMessage {
				normalExit.Store(true)
				return
			}

			if msgType != websocket.BinaryMessage {
				continue
			}

			if _, err = currentConsole.Write(msg); err != nil {
				log.Errorf("error while writing in console: %v", err)
				return
			}
		}
	}
}

func readUserConsole(ctx context.Context, cancel context.CancelFunc, currentConsole console.Console, stdIn chan []byte, normalExit *atomic.Bool, wg *sync.WaitGroup) {
	defer wg.Done()

	buffer := make([]byte, StdinBufferSize)
	// Persistent buffer to handle fragmented bracketed paste sequences
	var pendingBytes []byte

	for {
		if ctx.Err() != nil || normalExit.Load() {
			return
		}

		count, err := currentConsole.Read(buffer)
		if err != nil {
			log.Error("error while reading on console:", err)
			cancel()
			return
		}

		// Do not handle Ctrl^C in order to be able to kill commands inside the container
		// if count > 0 && buffer[0] == 3 { // Ctrl+C
		// 	log.Info("Detected Ctrl+C from user input, exiting gracefully...")
		//	cancel()
		//	return
		// }

		// Combine pending bytes from previous read with new data
		data := append(pendingBytes, buffer[0:count]...)

		// Handle fragmentation of bracketed paste sequences
		// Instead of filtering them out, we ensure they are sent complete
		toSend, pending := handleBracketedPasteFragmentation(data)
		pendingBytes = pending

		if len(toSend) > 0 {
			select {
			case <-ctx.Done():
				return
			case stdIn <- toSend:
			}
		}
	}
}

// handleBracketedPasteFragmentation ensures bracketed paste sequences are sent complete
// to prevent Terminal.app's fragmentation issues from corrupting the stream.
// If a potential sequence is incomplete at the end, it's buffered for the next read.
// Returns: (data to send, pending bytes that might be part of an incomplete sequence)
func handleBracketedPasteFragmentation(data []byte) ([]byte, []byte) {
	// Look for ESC at the end that could be the start of a bracketed paste sequence
	// The sequences are: ESC[200~ (start) and ESC[201~ (end)
	// We need to check if we have a potentially incomplete sequence at the end

	if len(data) == 0 {
		return data, nil
	}

	// Check if the end of data could be the start of a bracketed paste sequence
	// ESC[200~ or ESC[201~ are 6 bytes long
	for checkLen := 1; checkLen < 6 && checkLen <= len(data); checkLen++ {
		tail := data[len(data)-checkLen:]

		// Check if this could be the start of ESC[200~ or ESC[201~
		if isPotentialBracketedPastePrefix(tail) {
			// Buffer these bytes for the next read
			return data[:len(data)-checkLen], tail
		}
	}

	// No incomplete sequence detected, send everything
	return data, nil
}

// isPotentialBracketedPastePrefix checks if data could be the start of a bracketed paste sequence
func isPotentialBracketedPastePrefix(data []byte) bool {
	bracketedPasteStart := []byte{0x1b, '[', '2', '0', '0', '~'}
	bracketedPasteEnd := []byte{0x1b, '[', '2', '0', '1', '~'}

	if len(data) == 0 || len(data) >= 6 {
		return false
	}

	// Check if it matches the start of either sequence
	matchesStart := true
	matchesEnd := true

	for i := 0; i < len(data); i++ {
		if data[i] != bracketedPasteStart[i] {
			matchesStart = false
		}
		if data[i] != bracketedPasteEnd[i] {
			matchesEnd = false
		}
	}

	return matchesStart || matchesEnd
}
