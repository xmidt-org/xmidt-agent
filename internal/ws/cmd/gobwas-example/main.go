package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type ConnSpy struct {
	net.Conn
}

func (c *ConnSpy) Read(b []byte) (n int, err error) {
	fmt.Println("ConnSpy Read")
	n, err = c.Conn.Read(b)
	fmt.Printf("Read %d bytes\n", n)
	fmt.Printf("Buffer: \n%s\n", string(b))
	return n, err
}

func (c *ConnSpy) Write(b []byte) (n int, err error) {
	fmt.Println("ConnSpy Write")
	fmt.Printf("Buffer: \n%s\n", string(b))
	return c.Conn.Write(b)
}

func (c *ConnSpy) Close() error {
	fmt.Println("ConnSpy Close")
	return c.Conn.Close()
}

func (c *ConnSpy) LocalAddr() net.Addr {
	fmt.Println("ConnSpy LocalAddr")
	return c.Conn.LocalAddr()
}

func (c *ConnSpy) RemoteAddr() net.Addr {
	fmt.Println("ConnSpy RemoteAddr")
	return c.Conn.RemoteAddr()
}

func (c *ConnSpy) SetDeadline(t time.Time) error {
	t = time.Now().Add(time.Duration(5) * time.Second)
	fmt.Println("ConnSpy SetDeadline: ", time.Until(t))
	return c.Conn.SetDeadline(t)
}

func (c *ConnSpy) SetReadDeadline(t time.Time) error {
	fmt.Println("ConnSpy SetReadDeadline: ", time.Until(t))
	return c.Conn.SetReadDeadline(t)
}

func (c *ConnSpy) SetWriteDeadline(t time.Time) error {
	fmt.Println("ConnSpy SetWriteDeadline: ", time.Until(t))
	return c.Conn.SetWriteDeadline(t)
}

func main() {
	fmt.Println("Client started")
	for {
		dialer := ws.Dialer{
			Timeout: 10 * time.Second,
			Header:  ws.HandshakeHeaderString("X-Webpa-Device-Name: mac:112233445566"),
			OnStatusError: func(status int, reason []byte, resp io.Reader) {
				fmt.Printf("WTS Status error: %d %s\n", status, reason)
			},
			OnHeader: func(key, value []byte) error {
				fmt.Println("WTS OnHeader")
				fmt.Printf("Header: %s=%s\n", key, value)
				return nil
			},
			TLSClient: func(conn net.Conn, hostname string) net.Conn {
				fmt.Println("WTS TLSClient")
				fmt.Printf("Hostname: %s\n", hostname)
				return conn
			},
			WrapConn: func(conn net.Conn) net.Conn {
				fmt.Println("WTS Wrapping connection")
				return &ConnSpy{conn}
			},
		}

		/*
			dd := wsutil.DebugDialer{
				Dialer: dialer,
				OnRequest: func(req []byte) {
					fmt.Printf("Request: %s\n", req)
				},
					OnResponse: func(res []byte) {
						fmt.Printf("Response: %s\n", res)
					},
			}
		*/
		dd := dialer

		conn, _, hs, err := dd.Dial(context.Background(), "https://fabric.xmidt.comcast.net:8080/api/v2/device")
		fmt.Printf("Handshake: %+v\n", hs)
		if err != nil {
			fmt.Printf("Cannot connect: %v", err)
			time.Sleep(time.Duration(5) * time.Second)
			continue
		}
		fmt.Println("Connected to server")
		for i := 0; i < 10; i++ {
			randomNumber := strconv.Itoa(rand.Intn(100))
			msg := []byte(randomNumber)
			err = wsutil.WriteClientMessage(conn, ws.OpText, msg)
			if err != nil {
				fmt.Println("Cannot send: " + err.Error())
				continue
			}
			fmt.Println("Client message send with random number " + randomNumber)
			msg, _, err := wsutil.ReadServerData(conn)
			if err != nil {
				fmt.Println("Cannot receive data: " + err.Error())
				continue
			}
			fmt.Println("Server message received with random number: " + string(msg))
			time.Sleep(time.Duration(5) * time.Second)
		}
		err = conn.Close()
		if err != nil {
			fmt.Println("Cannot close the connection: " + err.Error())
			os.Exit(1)
		}
		fmt.Println("Disconnected from server")
	}
}
