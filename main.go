package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type MsgType string

const (
	TypeRequest  MsgType = "request"
	TypeResponse MsgType = "response"
	TypePing     MsgType = "ping"
	TypePong     MsgType = "pong"
)

type Envelope struct {
	Type    MsgType             `json:"type"`
	AppID   string              `json:"app_id,omitempty"`
	ReqID   string              `json:"req_id,omitempty"`
	Method  string              `json:"method,omitempty"`
	Path    string              `json:"path,omitempty"`
	Headers map[string][]string `json:"headers,omitempty"`
	Status  int                 `json:"status,omitempty"`
	Body    []byte              `json:"body,omitempty"`
	// 可扩展字段（重试、重放标志、时间戳等）
}

// CloneHeaders 复制 http.Header -> map[string][]string
func CloneHeaders(h http.Header) map[string][]string {
	out := make(map[string][]string, len(h))
	for k, v := range h {
		vv := make([]string, len(v))
		copy(vv, v)
		out[k] = vv
	}
	return out
}

// ToHTTP 写回 HTTP 响应
func (e *Envelope) ToHTTP(w http.ResponseWriter) {
	for k, v := range e.Headers {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}
	if e.Status == 0 {
		e.Status = http.StatusOK
	}
	w.WriteHeader(e.Status)
	_, _ = w.Write(e.Body)
}

func (e *Envelope) Marshal() ([]byte, error) { return json.Marshal(e) }
func (e *Envelope) Unmarshal(b []byte) error { return json.Unmarshal(b, e) }

func main() {
	// 定义命令行参数（只使用全称，但保持短称可用）
	var (
		server = flag.String("server", "", "relay server address (e.g., p8205-k3s-9.k3s-dev.myones.net)")
		appID  = flag.String("app", "", "application ID")
		token  = flag.String("token", "", "relay authorization token")
		port   = flag.String("port", "", "local target service port")
	)

	// 添加短称选项（不显示在帮助中）
	flag.StringVar(server, "s", "", "")
	flag.StringVar(appID, "a", "", "")
	flag.StringVar(token, "t", "", "")
	flag.StringVar(port, "p", "", "")

	// 设置使用说明
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of local_agent:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s [options]\n\n", flag.CommandLine.Name())
		fmt.Fprintf(flag.CommandLine.Output(), "Required Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nRecommended values:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  -s, --server: p8205-k3s-9.k3s-dev.myones.net\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  -a, --app: app_F63GRnbJR6xINLyK\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  -t, --token: testmyrelaytoken\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  -p, --port: 8082\n")
		fmt.Fprintf(flag.CommandLine.Output(), "\nExample:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s -s p8205-k3s-9.k3s-dev.myones.net -a app_F63GRnbJR6xINLyK -t testmyrelaytoken -p 8082\n", flag.CommandLine.Name())
	}

	flag.Parse()

	// 验证必填参数
	if *server == "" {
		fmt.Fprintf(flag.CommandLine.Output(), "Error: server address is required\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Use -s or --server to specify the relay server address\n\n")
		flag.Usage()
		return
	}
	if *appID == "" {
		fmt.Fprintf(flag.CommandLine.Output(), "Error: application ID is required\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Use -a or --app to specify the application ID\n\n")
		flag.Usage()
		return
	}
	if *token == "" {
		fmt.Fprintf(flag.CommandLine.Output(), "Error: authorization token is required\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Use -t or --token to specify the authorization token\n\n")
		flag.Usage()
		return
	}
	if *port == "" {
		fmt.Fprintf(flag.CommandLine.Output(), "Error: target service port is required\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Use -p or --port to specify the target service port\n\n")
		flag.Usage()
		return
	}

	// 构建 relay URL
	var relayURL string
	endpoint, err := url.Parse(*server)
	if err != nil {
		log.Fatalf("invalid server URL: %v", err)
	}
	if endpoint.Scheme == "http" {
		relayURL = fmt.Sprintf("ws://%s/platform/plugin_relay/app?app_id=%s", endpoint.Host, *appID)
	} else {
		relayURL = fmt.Sprintf("wss://%s/platform/plugin_relay/app?app_id=%s", endpoint.Host, *appID)
	}

	target := fmt.Sprintf("http://127.0.0.1:%s", *port)

	header := http.Header{}
	if *token != "" {
		header.Set("Relay-Authorization", "bearer "+*token)
	}
	log.Printf("[agent] dialing %s", relayURL)

	for {
		//log.Printf("[agent] dialing %s", relayURL)
		c, _, err := websocket.DefaultDialer.Dial(relayURL, header)
		if err != nil {
			//log.Printf("[agent] dial failed: %v; retrying in 2s", err)
			time.Sleep(2 * time.Second)
			continue
		}
		c.SetPingHandler(func(string) error {
			c.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(10*time.Second))
			return nil
		})
		log.Printf("[agent] dialed %s", relayURL)
		accessURL := fmt.Sprintf("%s://%s/platform/plugin_relay/app_dispatch/%s", endpoint.Scheme, endpoint.Host, *appID)
		log.Printf("you can use [%s] to access the target service", accessURL)
		if err := loop(c, target); err != nil {
			log.Printf("[agent] loop ended: %v, retrying in 2s", err)
		}
		c.Close()
		time.Sleep(1 * time.Second)
	}
}

func loop(c *websocket.Conn, base string) error {
	for {
		var env Envelope
		if err := c.ReadJSON(&env); err != nil {
			// 如果是超时，继续循环
			if strings.Contains(err.Error(), "timeout") ||
				strings.Contains(err.Error(), "deadline") {
				// 重置读取超时
				c.SetReadDeadline(time.Now().Add(60 * time.Second))
				continue
			}
			return err
		}

		if env.Type == TypePing {
			log.Printf("[agent] received ping")
			c.WriteJSON(&Envelope{Type: TypePong})
			continue
		}

		//log.Printf("[agent] received message type: %s", env.Type)
		if env.Type != TypeRequest {
			continue
		}

		resp := handleRequest(base, &env)
		if err := c.WriteJSON(resp); err != nil {
			return err
		}
	}
}

func handleRequest(base string, req *Envelope) *Envelope {
	u, _ := url.Parse(base)
	u.Path = joinPath(u.Path, req.Path)

	httpReq, _ := http.NewRequest(req.Method, u.String(), bytes.NewReader(req.Body))
	for k, v := range req.Headers {
		for _, vv := range v {
			httpReq.Header.Add(k, vv)
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return &Envelope{Type: TypeResponse, ReqID: req.ReqID, Status: http.StatusBadGateway, Headers: map[string][]string{"X-Agent-Error": {err.Error()}}, Body: []byte("agent request error")}
	}
	defer httpResp.Body.Close()
	b, _ := io.ReadAll(httpResp.Body)
	return &Envelope{
		Type:    TypeResponse,
		ReqID:   req.ReqID,
		Status:  httpResp.StatusCode,
		Headers: CloneHeaders(httpResp.Header),
		Body:    b,
	}
}

func joinPath(a, b string) string {
	as := strings.TrimSuffix(a, "/")
	bs := "/" + strings.TrimPrefix(b, "/")
	return as + bs
}
