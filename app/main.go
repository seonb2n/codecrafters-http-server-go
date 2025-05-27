package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

var _ = net.Listen
var _ = os.Exit

func main() {

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	defer l.Close()

	for {
		// 연결 수락
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			continue
		}

		// 각 연결을 고루틴으로 처리
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("Error reading request:", err.Error())
		return
	}

	request := string(buffer[:n])
	path := parseRequestPath(request)

	response := routeRequest(path)

	// 응답 전송
	_, err = conn.Write([]byte(response))
	if err != nil {
		fmt.Println("Error writing response:", err.Error())
		return
	}
}

func parseRequestPath(request string) string {
	lines := strings.Split(request, "\r\n")
	if len(lines) == 0 {
		return ""
	}

	parts := strings.Split(lines[0], " ")
	if len(parts) < 2 {
		return ""
	}

	return parts[1]
}

// routeRequest는 경로에 따라 적절한 HTTP 응답을 반환합니다
func routeRequest(path string) string {
	switch path {
	case "/":
		return "HTTP/1.1 200 OK\r\n\r\n"
	default:
		return "HTTP/1.1 404 Not Found\r\n\r\n"
	}
}
