package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
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

	response := routeRequest(request)

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

func parseHeaders(request string) map[string]string {
	headers := make(map[string]string)
	lines := strings.Split(request, "\r\n")

	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			break
		}
		// 헤더 파싱 (예: "User-Agent: foobar/1.2.3")
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			headerName := strings.ToLower(strings.TrimSpace(parts[0]))
			headerValue := strings.TrimSpace(parts[1])
			headers[headerName] = headerValue
		}
	}
	return headers
}

func parseUserAgent(request string) string {
	headers := parseHeaders(request)
	return headers["user-agent"]
}

// routeRequest는 경로에 따라 적절한 HTTP 응답을 반환합니다
func routeRequest(request string) string {
	path := parseRequestPath(request)

	switch {
	case path == "/":
		return "HTTP/1.1 200 OK\r\n\r\n"
	case strings.HasPrefix(path, "/echo/"):
		str := path[6:]
		return responseEcho(str)
	case strings.HasPrefix(path, "/user-agent"):
		userAgent := parseUserAgent(request)
		return responseUserAgent(userAgent)
	default:
		return "HTTP/1.1 404 Not Found\r\n\r\n"
	}
}

func responseEcho(str string) string {
	contentLength := len(str)
	return fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %s\r\n\r\n%s",
		strconv.Itoa(contentLength), str)
}

func responseUserAgent(userAgent string) string {
	contentLength := len(userAgent)
	return fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s",
		contentLength, userAgent)
}
