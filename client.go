package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"net/url"
	"os"
)

type msg struct {
	Sender    string `json:"sender"`
	Content   string `json:"content"`
	Recipient string `json:"recipient,omitempty"`
}

var addr = flag.String("addr", "127.0.0.1:9988", "http service address")
var uid string

func main() {
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/"}
	var dialer *websocket.Dialer
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Println("出错：", err)
		return
	}
	fmt.Println("")
	go input(conn)
	//类似于onmessage事件
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("read:", err)
			break
		}

		mess := msg{}
		if err := json.Unmarshal(message, &mess); err != nil {
			fmt.Println("出错", err)
			break
		}
		if mess.Content == "" {
			uid = mess.Sender
		} else {
			var fo string
			if mess.Sender == uid {
				fo = "我"
			} else {
				fo = mess.Sender
				if fo == "" {
					fo = "广播"
				}
			}
			fmt.Printf("%s: %s\n", fo, mess.Content)
		}
	}
}

//相当于创建一个绑定终端输入的事件
//将输入的内容发送到服务端
func input(conn *websocket.Conn) {
	for {
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadBytes('\n')
		conn.WriteMessage(websocket.TextMessage, []byte(input))
	}
}
