package pkg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
)

type LogRequest struct {
	ServiceID      utils.Id
	EnvironmentID  utils.Id
	ProjectID      utils.Id
	OrganizationID utils.Id
	ClusterID      utils.Id
	RawFormat      bool
	Download       bool
	OutputFile     string
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

// DownloadLogs downloads logs from the websocket and saves them to a file
func DownloadLogs(req *LogRequest) error {
	wsConn, err := createLogWebsocket(req)
	if err != nil {
		return fmt.Errorf("error while creating websocket connection: %w", err)
	}
	defer func() {
		if err := wsConn.Close(); err != nil {
			log.Error("error while closing websocket connection: ", err)
		}
	}()

	// Create output file
	file, err := os.Create(req.OutputFile)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Error("error closing file: ", err)
		}
	}()

	var logMessage LogMessage
	logCount := 0

	// Channel to receive messages
	msgChan := make(chan []byte, 10)
	errChan := make(chan error, 1)
	done := make(chan struct{})

	// Goroutine to read from websocket
	go func() {
		defer close(msgChan)
		for {
			select {
			case <-done:
				return
			default:
				_, msg, err := wsConn.ReadMessage()
				if err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						return
					}
					errChan <- err
					return
				}
				msgChan <- msg
			}
		}
	}()

	// Timeout for inactivity
	inactivityTimer := time.NewTimer(5 * time.Second)
	defer inactivityTimer.Stop()

	for {
		select {
		case msg, ok := <-msgChan:
			if !ok {
				// Channel closed, websocket is done
				fmt.Printf("\nDownload complete: %d log entries saved\n", logCount)
				return nil
			}

			// Reset inactivity timer
			if !inactivityTimer.Stop() {
				select {
				case <-inactivityTimer.C:
				default:
				}
			}
			inactivityTimer.Reset(5 * time.Second)

			logCount++

			// Format log line
			var logLine string
			if req.RawFormat {
				logLine = string(msg) + "\n"
			} else {
				if err := json.Unmarshal(msg, &logMessage); err != nil {
					log.Warn("Error parsing log message, writing raw: ", err)
					logLine = string(msg) + "\n"
				} else {
					logLine = fmt.Sprintf("| %s | %s | %s\n",
						logMessage.CreatedAt.Format("2006-01-02 15:04:05.000"),
						logMessage.PodName,
						logMessage.Message)
				}
			}

			// Write to file
			if _, err := file.WriteString(logLine); err != nil {
				close(done)
				return fmt.Errorf("error writing to file: %w", err)
			}

			// Print progress every 100 logs
			if logCount%100 == 0 {
				fmt.Printf("Downloaded %d log entries...\n", logCount)
			}

		case err := <-errChan:
			close(done)
			return fmt.Errorf("error reading from websocket: %w", err)

		case <-inactivityTimer.C:
			close(done)
			if logCount > 0 {
				fmt.Printf("\nDownload complete: %d log entries saved\n", logCount)
				return nil
			}
			return fmt.Errorf("timeout: no logs received within 5 seconds")
		}
	}
}
