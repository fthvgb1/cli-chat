package main

import (
	"github.com/gorilla/websocket"
	"encoding/json"
	"net/http"
	"github.com/satori/go.uuid"
	"fmt"
)

type ClientManager struct {
	clients    map[*client]bool
	broadcast  chan []byte
	register   chan *client
	unregister chan *client
}

type client struct {
	id     string
	socket *websocket.Conn
	send   chan []byte
}

type Message struct {
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	Content   string `json:"content,omitempty"`
}

var manager = ClientManager{
	make(map[*client]bool),
	make(chan []byte),
	make(chan *client),
	make(chan *client),
}

func (manager *ClientManager) send(message []byte, ignore *client) {
	for conn := range manager.clients {
		if conn != ignore {
			conn.send <- message
		}
	}
}

/*
相当于为每个client创建了一个事件
一旦可以从client.send获取数据，就可以向
该client发送信息
 */
func (c *client) write() {
	defer func() {
		c.socket.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				c.socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.socket.WriteMessage(websocket.TextMessage, msg)
		}
	}
}

//也是相当于为每个client创建了一个同onmessage类似的事件
//即只要client发送数据过来，即可向其它客户端推送的处理
func (c *client) read() {
	defer func() {
		manager.unregister <- c
		c.socket.Close()
	}()
	for {
		_, message, err := c.socket.ReadMessage()
		if err != nil {
			manager.unregister <- c
			c.socket.Close()
			break
		}
		jsonMessage, _ := json.Marshal(&Message{Sender: c.id, Content: string(message)})
		manager.broadcast <- jsonMessage
	}
}

func (manager *ClientManager) start() {
	for {
		select {
		case conn := <-manager.register:
			manager.clients[conn] = true
			jsonMessage, _ := json.Marshal(&Message{Content: "一个新客户端接入"})
			manager.send(jsonMessage, conn)
		case conn := <-manager.unregister:
			if _, ok := manager.clients[conn]; ok {
				close(conn.send)
				delete(manager.clients, conn)
				jsonMessage, _ := json.Marshal(&Message{Content: "一个新客户端下线了"})
				manager.send(jsonMessage, conn)
			}
		case message := <-manager.broadcast:
			for conn := range manager.clients {
				select {
				//向正常连接的客户推送消息
				case conn.send <- message:
				default:
					//不能推送的就闭关该连接
					close(conn.send)
					delete(manager.clients, conn)
				}
			}

		}
	}
}

func main() {
	go manager.start()
	http.HandleFunc("/", wsPage)
	http.ListenAndServe(":9988", nil)
}

func wsPage(res http.ResponseWriter, r *http.Request) {
	conn, err := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		return true
	}}).Upgrade(res, r, nil)
	if err != nil {
		http.NotFound(res, r)
		return
	}
	id, err := uuid.NewV4()
	if err != nil {
		http.NotFound(res, r)
		return
	}
	uid := fmt.Sprintf("%s", id)
	client := &client{id: uid, socket: conn, send: make(chan []byte)}
	info, _ := json.Marshal(Message{Sender: uid})
	conn.WriteMessage(websocket.TextMessage, []byte(info))
	manager.register <- client
	go client.read()
	go client.write()
}
