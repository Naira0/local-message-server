package main

import (
	"encoding/binary"
	"net"
	"strconv"
	"unsafe"
)

func byteID(s string) ([]byte, error) {
	asInt, err := strconv.ParseUint(s, 10, 64)

	if err != nil {
		return nil, err
	}

	buff := make([]byte, unsafe.Sizeof(asInt))

	binary.BigEndian.PutUint64(buff, uint64(asInt))

	return buff, nil
}

func getIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")

	fatalErr(err, "Could not get up address")

	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func hasIP(ip net.IP) bool {
	net := net.IPNet{IP: ip, Mask: ip.DefaultMask()}
	return net.Contains(ip)
}
