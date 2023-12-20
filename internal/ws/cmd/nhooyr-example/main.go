package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/k0kubun/pp"
	"nhooyr.io/websocket"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	opt := websocket.DialOptions{
		HTTPHeader: http.Header{
			"X-Webpa-Device-Name": []string{"mac:112233445566"},
		},
	}

	c, resp, err := websocket.Dial(ctx, "wss://fabric.xmidt.comcast.net:8080/api/v2/device", &opt)
	if err != nil {
		panic(err)
	}
	defer c.Close(websocket.StatusInternalError, "the sky is falling")

	fmt.Printf("Response: %s\n", resp.Status)
	//pp.Println(resp)

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	c.SetPingCallback(func() {
		fmt.Println("got ping request")
	})
	mt, buf, err := c.Read(ctx)

	pp.Println("got", mt, buf, err)

	c.Close(websocket.StatusNormalClosure, "")
}
