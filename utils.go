package holepunch

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type NoIPv4FoundError struct{}

type peerInfo struct {
	ip     net.IP
	port   uint16
	isIPv6 bool
}

func (p *peerInfo) portToString() string {
	return strconv.FormatInt(int64(p.port), 10)
}

func (p peerInfo) String() string {
	if p.isIPv6 {
		return "[" + p.ip.String() + "]" + ":" + p.portToString()
	}

	return p.ip.String() + ":" + p.portToString()
}

func (e NoIPv4FoundError) Error() string {
	return "No IPv4 Interface found"
}

func addressStringToIP(address string) net.IP {
	split := strings.Split(address, "/")
	ipSlice := strings.Split(split[0], ".")

	parts := make([]byte, 4)

	for i := range ipSlice {
		part, _ := strconv.ParseInt(ipSlice[i], 10, 16)
		parts[i] = byte(part)
	}

	ip := net.IPv4(parts[0], parts[1], parts[2], parts[3])

	return ip

}

func getMyIpv4Addr() (net.IP, error) {
	ifaces, _ := net.Interfaces()

	for _, iface := range ifaces {

		addr, _ := iface.Addrs()

		for _, a := range addr {
			if strings.Contains(a.String(), ":") { //must be an ipv6
				continue
			}

			ip := addressStringToIP(a.String())

			if ip.IsLoopback() {
				continue
			}

			return ip, nil

		}
	}
	e := NoIPv4FoundError{}
	return nil, e
}

func ipToBytes(ip string) []byte {
	split := strings.Split(ip, ".")

	parts := make([]byte, 4)
	for i := range split {
		part, _ := strconv.ParseInt(split[i], 10, 16)

		parts[i] = byte(part)
	}

	return parts

}

func fromIntToBytes(num uint16) ([]byte, error) {
	buffer := new(bytes.Buffer)

	err := binary.Write(buffer, binary.BigEndian, num)

	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

//  1 byte   length prefixed & zero terminated  4 bytes 16 bytes
// [  rpc   ][            uid                ] [  ip  ] [  port  ]
//
func createSessionPacket(uid, address, port string) []byte {
	packet := make([]byte, 0)
	p, _ := strconv.ParseInt(port, 10, 16)

	packet = append(packet, byte(CREATE_SESSION))
	//uid
	packet = append(packet, byte(len(uid)))
	packet = append(packet, []byte(uid)...)
	packet = append(packet, byte(0))
	//ip
	packet = append(packet, ipToBytes(address)...)
	packet = append(packet, byte(0))
	//port
	portBytes, _ := fromIntToBytes(uint16(p))
	packet = append(packet, portBytes...)
	packet = append(packet, ETX)

	return packet
}

func createConnRequestPacket(peer string) []byte {
	packet := make([]byte, 0)

	packet = append(packet, byte(CONN_REQUEST))
	packet = append(packet, byte(len(peer)))
	packet = append(packet, []byte(peer)...)
	packet = append(packet, ETX)

	return packet
}

func parseConnRequestResponse(data []byte) (string, bool) {
	var ok bool

	if data[1] == NAK {
		ok = false
	} else {
		ok = true
	}

	peerLength := data[2]
	peer := string(data[2 : 2+peerLength])

	return peer, ok

}

func parseInitHolePunchMessage(data []byte) peerInfo {
	var isIPv6 bool
	var ip string
	var startOfPort int

	if data[1] == 1 {
		isIPv6 = true
		ip = parseIPv6(data[2:10])
		startOfPort = 10

	} else {
		isIPv6 = false
		ip = parseIPv4(data[2:6])
		startOfPort = 6

	}

	port := binary.BigEndian.Uint16(data[startOfPort : startOfPort+2])

	peer := peerInfo{
		port:   port,
		ip:     net.ParseIP(ip),
		isIPv6: isIPv6,
	}

	return peer
}

func parseIPv4(ip []byte) string {

	buf := new(bytes.Buffer)

	for i := range ip {
		bt := int(ip[i])
		buf.WriteString(strconv.Itoa(bt))

		if i < 3 {
			buf.WriteString(".")
		}
	}

	return buf.String()
}

func parseIPv6(ip []byte) string {
	buf := new(bytes.Buffer)

	for i := 0; i < len(ip); i += 2 {

		part := binary.BigEndian.Uint16(ip[i : i+2])
		buf.WriteString(fmt.Sprintf("%X", part))

		if i != 14 {
			buf.WriteString(":")
		}
	}

	return buf.String()
}
