package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
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
		cli.StringFlag{
			Name:  "relay",
			Value: "",
			Usage: "",
		},
	}
	app.Action = func(cli *cli.Context) {

		cred := strings.SplitN(cli.String("user"), "=", 2)
		host := cli.String("host")
		port := cli.Int("port")
		realm := cli.String("realm")

		// Prepare the IP to connect to
		addr := &net.UDPAddr{IP: net.ParseIP(host), Port: port}

		// Generate a certificate and private key to secure the connection
		certificate, genErr := selfsign.GenerateSelfSigned()
		util.Check(genErr)

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

		defer func() {
			util.Check(dtlsConn.Close())
		}()

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

		util.WriteFile("relay.port", relayConn.LocalAddr().String())

		if cli.Bool("ping") {
			err = doPingTest(client, relayConn)
			if err != nil {
				fmt.Println("doPingerr", err)
				panic(err)
			}
		}
	}
	err := app.Run(os.Args)
	if err != nil {
		fmt.Println("app run error:", err)
	}
}

func doPingTest(client *turn.Client, relayConn net.PacketConn) error {
	// Send BindingRequest to learn our external IP
	mappedAddr, err := client.SendBindingRequest()
	fmt.Println("mappedAddr", mappedAddr)
	if err != nil {
		return err
	}

	// Set up pinger socket (pingerConn)
	pingerConn, err := net.ListenPacket("udp4", "0.0.0.0:0")
	if err != nil {
		panic(err)
	}
	defer func() {
		if closeErr := pingerConn.Close(); closeErr != nil {
			panic(closeErr)
		}
	}()
	//Punch a UDP hole for the relayConn by sending a data to the mappedAddr.
	//This will trigger a TURN client to generate a permission request to the
	//TURN server. After this, packets from the IP address will be accepted by
	//the TURN server.
	_, err = relayConn.WriteTo([]byte("Hello world"), mappedAddr)
	if err != nil {
		return err
	}

	// Start read-loop on pingerConn
	go func() {
		buf := make([]byte, 1600)
		i := 0
		for {
			i++
			n, from, pingerErr := pingerConn.ReadFrom(buf)
			if pingerErr != nil {
				break
			}

			msg := string(buf[:n])
			if sentAt, pingerErr := time.Parse(time.RFC3339Nano, msg); pingerErr == nil {
				rtt := time.Since(sentAt)
				log.Printf("%d:%d bytes from from %s time=%d ms\n", i, n, from.String(), int(rtt.Seconds()*1000))
			}

		}
	}()
	go func() {

		buf := make([]byte, 1600)
		log.Println("relayConn:", relayConn.LocalAddr())
		for {
			n, from, readerErr := relayConn.ReadFrom(buf)

			if readerErr != nil {
				log.Println("readerErr:", readerErr)
				break
			}

			// Echo back
			if _, readerErr = relayConn.WriteTo(buf[:n], from); readerErr != nil {
				log.Println("err:", readerErr)
				break
			}
		}
	}()
	time.Sleep(500 * time.Millisecond)
	// Send 10 packets from relayConn to the echo server
	for i := 0; i < 10; i++ {
		msg := time.Now().Format(time.RFC3339Nano)
		_, err = pingerConn.WriteTo([]byte(msg), relayConn.LocalAddr())

		// For simplicity, this example does not wait for the pong (reply).
		// Instead, sleep 1 second.
		time.Sleep(time.Second)
	}

	return nil
}
