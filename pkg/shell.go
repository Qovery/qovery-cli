package pkg

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	"golang.org/x/term"
)

const StdinBufferSize = 4096
const ReconnectDelay = 5 * time.Second
const PingInterval = 30 * time.Second

// ReadTimeout must be > 2 × PingInterval so that a healthy connection always receives a pong
// before the deadline fires. The pong handler resets the deadline on every pong received.
const ReadTimeout = 75 * time.Second

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
	EphemeralMode  string   `url:"mode,omitempty"`
	CpuOverride    string   `url:"cpu_override,omitempty"`
	MemoryOverride string   `url:"memory_override,omitempty"`
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

	// Allocate a PTY only when stdin is a real terminal. When piped (e.g.
	// `qovery shell --command ... < input` or invoked from automation),
	// containerd/console.Current() panics with "provided file is not a console".
	// In that case we behave like `kubectl exec -i`: pipe os.Stdin/os.Stdout
	// straight through and leave TtyWidth/TtyHeight at zero so the server
	// does not attempt to allocate a TTY on the remote side either.
	interactive := term.IsTerminal(int(os.Stdin.Fd()))

	var stdinReader io.Reader = os.Stdin
	var stdoutWriter io.Writer = os.Stdout

	if interactive {
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

		stdinReader = currentConsole
		stdoutWriter = currentConsole
	}

	stdIn := make(chan []byte)
	wg.Add(1)
	go readUserConsole(ctx, cancel, stdinReader, interactive, stdIn, &normalExit, &wg)

	for {
		if ctx.Err() != nil || userCancelled.Load() || normalExit.Load() {
			log.Info("Shell exited, not reconnecting.")
			break
		}

		log.Info("Attempting to (re)connect")
		var requestId string

		wsConn, resp, err := createWebsocketConn(req, path)
		if resp != nil {
			requestId = resp.Header.Get("X-Qovery-Request-Id")
			log.Info("Connected to shell with requestId: ", requestId)
		}
		if err != nil {
			log.Errorf("WebSocket connection failed: %v %s", err, requestId)
			if ctx.Err() != nil || userCancelled.Load() || normalExit.Load() {
				log.Info("User cancelled or shell exited during connection attempt.")
				break
			}
			time.Sleep(ReconnectDelay)
			continue
		}
		done := make(chan struct{})
		wg.Add(1)
		go readWebsocketConnection(ctx, cancel, wsConn, requestId, stdoutWriter, done, &normalExit, &wg)

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
		if ctx.Err() == nil && !normalExit.Load() && !userCancelled.Load() {
			time.Sleep(ReconnectDelay)
		}
	}

	wg.Wait()
}

func createWebsocketConn(req interface{}, path string) (*websocket.Conn, *http.Response, error) {
	command, err := query.Values(req)
	if err != nil {
		return nil, nil, err
	}

	wsURL, err := url.Parse(fmt.Sprintf("%s%s", utils.WebsocketUrl(), path))
	if err != nil {
		return nil, nil, err
	}
	pattern := regexp.MustCompile("%5B([0-9]+)%5D=")
	wsURL.RawQuery = pattern.ReplaceAllString(command.Encode(), "[${1}]=")

	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		return nil, nil, err
	}

	headers := http.Header{"Authorization": {utils.GetAuthorizationHeaderValue(tokenType, token)}}
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL.String(), headers)
	return conn, resp, err
}

func readWebsocketConnection(ctx context.Context, cancel context.CancelFunc, wsConn *websocket.Conn, requestId string, out io.Writer, done chan struct{}, normalExit *atomic.Bool, wg *sync.WaitGroup) {
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

	// Set an initial read deadline. The pong handler refreshes it on every
	// pong so that idle-but-healthy sessions are not torn down; only truly
	// dead connections (no pong for ReadTimeout) are detected and closed.
	_ = wsConn.SetReadDeadline(time.Now().Add(ReadTimeout))
	// SetReadDeadline failure in the pong handler would surface as a ReadMessage error on
	// the next iteration, but cannot happen on a healthy net.Conn.
	wsConn.SetPongHandler(func(string) error {
		return wsConn.SetReadDeadline(time.Now().Add(ReadTimeout))
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msgType, msg, err := wsConn.ReadMessage()
			if err != nil {
				var e *websocket.CloseError
				if !errors.As(err, &e) {
					log.Errorf("error while reading on websocket %s: %v  ", requestId, err)
					return
				}
				switch {
				case e.Code == websocket.CloseNormalClosure:
					log.Info("** shell terminated bye **")
					normalExit.Store(true)
				case e.Code == 1007 || e.Code == 1008: // same as IsPermanentCloseError
					log.Errorf("Shell connection %s rejected: check your permissions or run 'qovery auth'", requestId)
					cancel()
				case IsAgentResponseTimeout(err): // must come before generic 1011 branch
					log.Warnf("Shell session %s timed out while the agent was preparing your connection. Retrying...", requestId)
				case e.Code == 1011:
					log.Warnf("%s Closing %s and Retrying...", ServiceUnavailableMessage("Shell"), requestId)
				default:
					log.Errorf("connection %s closed by server: %v", requestId, e)
				}
				return
			}

			if msgType == websocket.CloseMessage {
				normalExit.Store(true)
				return
			}

			if msgType != websocket.BinaryMessage {
				continue
			}

			if _, err = out.Write(msg); err != nil {
				log.Errorf("error while writing in console: %v", err)
				return
			}
		}
	}
}

func readUserConsole(ctx context.Context, cancel context.CancelFunc, in io.Reader, interactive bool, stdIn chan []byte, normalExit *atomic.Bool, wg *sync.WaitGroup) {
	defer wg.Done()

	buffer := make([]byte, StdinBufferSize)
	// Persistent buffer to handle fragmented bracketed paste sequences
	var pendingBytes []byte

	for {
		if ctx.Err() != nil || normalExit.Load() {
			return
		}

		count, err := in.Read(buffer)
		if err != nil {
			// In non-interactive mode (piped stdin), EOF means the input stream
			// is exhausted but the remote command may still produce output. We
			// flush any buffered bytes, then send EOT (0x04) so the remote PTY
			// line discipline propagates EOF to the remote process. Without
			// this, commands like `cat < file` or `sh -s < script.sh` hang
			// forever. The websocket loop stays open so we can drain remote
			// stdout until the server closes the session.
			if !interactive && errors.Is(err, io.EOF) {
				if len(pendingBytes) > 0 {
					select {
					case <-ctx.Done():
						return
					case stdIn <- pendingBytes:
					}
				}
				select {
				case <-ctx.Done():
				case stdIn <- []byte{0x04}:
				}
				return
			}
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
