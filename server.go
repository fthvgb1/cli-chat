package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"net/http"
	"os"
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
	//捕捉通道的通信
	for {
		select {
		//捕获是新链接
		case conn := <-manager.register:
			manager.clients[conn] = true
			jsonMessage, _ := json.Marshal(&Message{Content: "一个新客户端接入"})
			fmt.Println("一个新客户端接入")
			manager.send(jsonMessage, conn)
		//捕获的是链接下线
		case conn := <-manager.unregister:
			if _, ok := manager.clients[conn]; ok {
				close(conn.send)
				delete(manager.clients, conn)
				jsonMessage, _ := json.Marshal(&Message{Content: "一个新客户端下线了"})
				manager.send(jsonMessage, conn)
			}
		//捕获的是用户发送了消息
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
	//第一个新接入的客户端都执行如下流程
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
	//链接升级为websocket成功
	uid := fmt.Sprintf("%s", id)
	//创建一个client的结构体，将链接保存在这个结构体中
	client := &client{id: uid, socket: conn, send: make(chan []byte)}
	info, _ := json.Marshal(Message{Sender: uid})
	conn.WriteMessage(websocket.TextMessage, []byte(info))
	manager.register <- client
	//为每个链接都创建一个接收消息和推送消息的协程(事件)
	go client.read()
	go client.write()
	go func() {
		for {
			reader := bufio.NewReader(os.Stdin)
			message, _ := reader.ReadBytes('\n')
			jsonMessage, _ := json.Marshal(&Message{Sender: "管理员", Content: string(message)})
			manager.broadcast <- jsonMessage

		}
	}()
}
