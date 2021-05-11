package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/codepher/turn-dtls-client/util"
	"github.com/pion/dtls/v2"
	"github.com/pion/dtls/v2/pkg/crypto/selfsign"
	"github.com/pion/logging"
	"github.com/pion/turn/v2"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "turn-dtls-client"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "host",
			Value: "192.168.21.4",
			Usage: "TURN Server name.",
			//Required: true,
		},
		cli.IntFlag{
			Name:  "port,p",
			Value: 3478,
			Usage: "Listening port.",
			//Required: true,
		},
		cli.StringFlag{
			Name:  "user,u",
			Value: "lg=123",
			Usage: "A pair of username and password (e.g. \"user=pass\")",
		},
		cli.StringFlag{
			Name:  "realm",
			Value: "pion.ly",
			Usage: "Realm (defaults to \"pion.ly\")",
		},
		cli.BoolFlag{
			Name: "ping",
		},
	}
	app.Action = func(cli *cli.Context) {
		cred := strings.SplitN(cli.String("user"), "=", 2)
		host := cli.String("host")
		port := cli.Int("port")
		realm := cli.String("realm")
		ping := cli.Bool("ping")

		// 初始化 dtls
		// Prepare the IP to connect to
		addr := &net.UDPAddr{IP: net.ParseIP(host), Port: port}

		// Generate a certificate and private key to secure the connection
		certificate, genErr := selfsign.GenerateSelfSigned()
		util.Check(genErr)

		//
		// Everything below is the pion-DTLS API! Thanks for using it ❤️.
		//
		// Prepare the configuration of the DTLS connection
		config := &dtls.Config{
			Certificates:         []tls.Certificate{certificate},
			InsecureSkipVerify:   true,
			ExtendedMasterSecret: dtls.RequestExtendedMasterSecret,
			LoggerFactory:        logging.NewDefaultLoggerFactory(),
		}

		// Connect to a DTLS server
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		dtlsConn, err := dtls.DialWithContext(ctx, "udp", addr, config)
		util.Check(err)
		defer func() {
			util.Check(dtlsConn.Close())
		}()

		// 初始化 turn
		turnServerAddr := fmt.Sprintf("%s:%d", host, port)
		cfg := &turn.ClientConfig{
			STUNServerAddr: turnServerAddr,
			TURNServerAddr: turnServerAddr,
			Conn:           NewUdpDtls(dtlsConn),
			Username:       cred[0],
			Password:       cred[1],
			Realm:          realm,
			LoggerFactory:  logging.NewDefaultLoggerFactory(),
		}

		client, err := turn.NewClient(cfg)
		if err != nil {
			panic(err)
		}
		defer client.Close()

		// Start listening on the conn provided.
		err = client.Listen()
		if err != nil {
			panic(err)
		}

		// Allocate a relay socket on the TURN server. On success, it
		// will return a net.PacketConn which represents the remote
		// socket.
		// turn server 返回的链接通道
		relayConn, err := client.Allocate()
		if err != nil {
			panic(err)
		}
		defer func() {
			if closeErr := relayConn.Close(); closeErr != nil {
				panic(closeErr)
			}
		}()
		// The relayConn's local address is actually the transport
		// address assigned on the TURN server.
		log.Printf("relayed-address=%s", relayConn.LocalAddr().String())

		mappedAddr, err := client.SendBindingRequest()
		util.Check(err)
		_, err = relayConn.WriteTo([]byte("Hello world"), mappedAddr)
		util.Check(err)
		if !ping {
			util.WriteFile("relay.port", relayConn.LocalAddr().String())
			go ReadBind(relayConn, "relayc.port")
			util.Check(err)
		} else {
			util.WriteFile("relayc.port", relayConn.LocalAddr().String())
			relay, err := os.ReadFile("relay.port")

			if err != nil {
				util.Check(err)
			}
			addr := Bind(relayConn, string(relay))

			go pingTo(addr, relayConn)
		}

		if !ping {
			buf := make([]byte, 1600)
			log.Println("relayConn:", relayConn.LocalAddr())
			for {
				n, from, readerErr := relayConn.ReadFrom(buf)

				if readerErr != nil {
					log.Println("readerErr:", readerErr)
					break
				}
				log.Println("relayConn:msg:", string(buf[:n]))
				log.Println("relayConn:from:", from.String())

				// Echo back
				if _, readerErr = relayConn.WriteTo(buf[:n], from); readerErr != nil {
					log.Println("err:", readerErr)
					break
				}
			}
		} else {
			buf := make([]byte, 1600)
			i := 0
			for {
				i++
				n, from, pingerErr := relayConn.ReadFrom(buf)
				if pingerErr != nil {
					break
				}

				msg := string(buf[:n])
				fmt.Println("get msg", msg)
				if sentAt, pingerErr := time.Parse(time.RFC3339Nano, msg); pingerErr == nil {
					rtt := time.Since(sentAt)
					log.Printf("%d:%d bytes from from %s time=%d ms\n", i, n, from.String(), int(rtt.Seconds()*1000))
				}

			}

		}

	}
	err := app.Run(os.Args)
	if err != nil {
		fmt.Println("app run error:", err)
	}
}
func pingTo(addr net.Addr, relayConn net.PacketConn) {

	for i := 0; i < 10; i++ {
		msg := time.Now().Format(time.RFC3339Nano)
		fmt.Println("send msg:", msg)
		fmt.Println("add:", addr.Network(), addr.String())
		_, err := relayConn.WriteTo([]byte(msg), addr)
		if err != nil {
			panic(err)
		}
		// For simplicity, this example does not wait for the pong (reply).
		// Instead, sleep 1 second.
		time.Sleep(time.Second)
	}
	return
}

// 动态绑定
func ReadBind(relayConn net.PacketConn, filename string) {
	oldRelay := ""
	for {
		relay, err := os.ReadFile(filename)
		util.Check(err)

		if oldRelay != string(relay) {
			fmt.Println("new relay", string(relay))
			oldRelay = string(relay)
			Bind(relayConn, oldRelay)
		}
		time.Sleep(time.Second)
	}
}
func Bind(relayConn net.PacketConn, relay string) net.Addr {

	cred := strings.SplitN(relay, ":", 2)
	port, _ := strconv.Atoi(cred[1])
	fmt.Println("port", port)
	add := &net.UDPAddr{IP: net.ParseIP(cred[0]), Port: port}
	fmt.Println("add", add.String())
	_, err := relayConn.WriteTo([]byte("Hello world"), add)
	util.Check(err)
	return add
}
