package holepunch

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/libp2p/go-reuseport"
)

func wgDone(wg *sync.WaitGroup, name string) {
	fmt.Println("Exit " + name)
	wg.Done()
}

func listenLoop(wg *sync.WaitGroup, addr *net.TCPAddr, doneChan chan int, connReadyChan chan net.Conn, closeChan chan int) {

	l, err := reuseport.Listen("tcp", addr.IP.String()+":"+strconv.Itoa(addr.Port))
	if err != nil {
		fmt.Println("Could not listen\n", err)
		return
	}

	fmt.Println("Listening for Peers at: ", addr)
	wg.Add(1)
	acceptedPeerConn := listen(l)

	go func() {
		defer wgDone(wg, "Listen Loop")

		select {
		case <-closeChan:
			return
		case conn := <-acceptedPeerConn:
			doneChan <- 1
			connReadyChan <- conn
		}

	}()

}

func listen(listener net.Listener) chan net.Conn {
	result := make(chan net.Conn)

	go func(connReady chan net.Conn) {

		conn, err := listener.Accept()

		if err != nil {
			fmt.Println("Error accepting....\nExiting listen loop")
			return
		}

		connReady <- conn

	}(result)

	return result
}

func readLoop(wg *sync.WaitGroup, reader *bufio.Reader, stopRequestChan chan int, initHolepunchChan chan peerInfo, closeChan chan int) {
	wg.Add(1)

	go func() {
		defer wgDone(wg, "Read Loop")
		packetReady := packetAvailable(reader)

		for {
			select {
			case <-closeChan:
				return
			case packet := <-packetReady:
				opType := packet[0]

				switch opType {
				case CONN_REQUEST_RESPONSE:
					//check if the relay server knows about the peer
					if _, ok := parseConnRequestResponse(packet); ok {
						stopRequestChan <- 1
					}
				case INIT_HOLEPUNCH:
					initHolepunchChan <- parseInitHolePunchMessage(packet)
				}
			}

		}
	}()
}

func packetAvailable(reader *bufio.Reader) chan []byte {
	result := make(chan []byte)

	go func(pkt chan []byte) {
		for {
			msg, err := reader.ReadString(ETX)

			if err != nil {
				fmt.Println("Error reading")
				return
			}

			packet := []byte(msg)
			pkt <- packet
		}
	}(result)

	return result
}

func connRequestLoop(wg *sync.WaitGroup, writer *bufio.Writer, connRequestChan chan string, stopRequestChan chan int, closeChan chan int) {
	wg.Add(1)

	ticker := time.NewTicker(time.Duration(3) * time.Second)
	var req string

	go func() {
		defer wgDone(wg, "Conn Request Loop")

		for {
			select {
			case <-closeChan:
				return
			case <-stopRequestChan:
				fmt.Println("Stopping: connectRequestLoop")
				ticker.Stop()
			case request := <-connRequestChan:
				req = request
				writer.Write(createConnRequestPacket(req))
				writer.Flush()

			case <-ticker.C:
				if req != "" {
					writer.Write(createConnRequestPacket(req))
					writer.Flush()
				}
			}
		}
	}()
}

func initHolepunch(wg *sync.WaitGroup, laddr string, initHolepunchChan chan peerInfo, stopListenChan chan int, connReadyChan chan net.Conn, closeChan chan int) {
	wg.Add(1)

	ticker := time.NewTicker(time.Duration(2) * time.Second)
	var peer *peerInfo

	go func() {
		defer wgDone(wg, "Init Holepunch Loop")

		for {
			select {
			case <-closeChan:
				return
			case <-stopListenChan:
				ticker.Stop()
			case peerInf := <-initHolepunchChan:
				peer = &peerInf
				conn, err := reuseport.Dial("tcp", laddr, peer.String())

				if err != nil {
					fmt.Println(err)
				} else {
					fmt.Println("we got a connection ", conn)
					ticker.Stop()
					connReadyChan <- conn
				}

			case <-ticker.C:

				if peer != nil {
					conn, err := reuseport.Dial("tcp", laddr, peer.String())

					if err != nil {
						fmt.Println(err)
					} else {
						fmt.Println("we got a connection ", conn)
						ticker.Stop()
						connReadyChan <- conn
					}
				}

			}
		}

	}()
}
