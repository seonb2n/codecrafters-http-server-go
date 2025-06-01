package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type ContentType int

const (
	ContentTypeNone ContentType = iota
	ContentTypeTextPlain
	ContentTypeApplicationOctetStream
	ContentTypeApplicationJSON
	ContentTypeTextHTML
)

func (ct ContentType) String() string {
	switch ct {
	case ContentTypeTextPlain:
		return "text/plain"
	case ContentTypeApplicationOctetStream:
		return "application/octet-stream"
	case ContentTypeApplicationJSON:
		return "application/json"
	case ContentTypeTextHTML:
		return "text/html"
	default:
		return ""
	}
}

var _ = net.Listen
var _ = os.Exit

var filesDirectory string

func main() {

	parseCommandLineArgs()

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

func parseCommandLineArgs() {
	args := os.Args
	for i, arg := range args {
		if arg == "--directory" && i+1 < len(args) {
			filesDirectory = args[i+1]
			return
		}
	}
	// 기본값 설정 (optional)
	filesDirectory = ""
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	for {
		buffer := make([]byte, 1024)
		n, err := conn.Read(buffer)
		if err != nil {
			// 연결이 닫혔거나 에러가 발생한 경우 루프 종료
			break
		}

		request := string(buffer[:n])

		// 빈 요청인 경우 건너뛰기
		if strings.TrimSpace(request) == "" {
			continue
		}

		// Connection: close 헤더 확인
		shouldClose := shouldCloseConnection(request)

		response := routeRequest(request)

		// Connection: close 헤더가 있으면 응답에 추가
		if shouldClose {
			response = addConnectionCloseHeader(response)
		}

		// 응답 전송
		_, err = conn.Write([]byte(response))
		if err != nil {
			break
		}

		// Connection: close가 있거나 HTTP/1.0인 경우 연결 종료
		if shouldClose {
			break
		}
	}
}

func shouldCloseConnection(request string) bool {
	headers := parseHeaders(request)
	connectionHeader := strings.ToLower(headers["connection"])
	return connectionHeader == "close" || isHTTP10(request)
}

// addConnectionCloseHeader는 응답에 Connection: close 헤더를 추가합니다
func addConnectionCloseHeader(response string) string {
	// HTTP 상태 라인과 헤더들을 분리
	parts := strings.Split(response, "\r\n\r\n")
	if len(parts) < 2 {
		return response
	}

	headerPart := parts[0]
	bodyPart := parts[1]

	// Connection: close 헤더 추가
	headerPart += "\r\nConnection: close"

	return headerPart + "\r\n\r\n" + bodyPart
}

func isHTTP10(request string) bool {
	lines := strings.Split(request, "\r\n")
	if len(lines) == 0 {
		return false
	}

	parts := strings.Split(lines[0], " ")
	if len(parts) < 3 {
		return false
	}

	return parts[2] == "HTTP/1.0"
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

func parseRequestMethod(request string) string {
	lines := strings.Split(request, "\r\n")
	if len(lines) == 0 {
		return ""
	}
	parts := strings.Split(lines[0], " ")
	if len(parts) < 1 {
		return ""
	}
	return parts[0]
}

func parseRequestBody(request string) string {
	bodyStart := strings.Index(request, "\r\n\r\n")
	if bodyStart == -1 {
		return ""
	}
	return request[bodyStart+4:]
}

func supportsGzip(headers map[string]string) bool {
	acceptEncoding := headers["accept-encoding"]
	if acceptEncoding == "" {
		return false
	}

	encodings := strings.Split(acceptEncoding, ",")
	for _, encoding := range encodings {
		if strings.TrimSpace(encoding) == "gzip" {
			return true
		}
	}
	return false
}

func compressGzip(data string) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write([]byte(data))
	if err != nil {
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func handleResponse(statusCode int, contentType ContentType, body string, useGzip bool) string {
	var status string
	switch statusCode {
	case 200:
		status = "HTTP/1.1 200 OK"
	case 201:
		status = "HTTP/1.1 201 Created"
	case 404:
		status = "HTTP/1.1 404 Not Found"
	case 500:
		status = "HTTP/1.1 500 Internal Server Error"
	default:
		status = "HTTP/1.1 200 OK"
	}

	// 헤더 구성
	var headers []string
	var responseBody string

	if useGzip && body != "" {
		compressedData, err := compressGzip(body)
		if err != nil {
			fmt.Println("Error compressing data: ", err.Error())
			responseBody = body
		} else {
			responseBody = string(compressedData)
			headers = append(headers, "Content-Encoding: gzip")
		}
	} else {
		responseBody = body
	}

	if contentType != ContentTypeNone {
		headers = append(headers, fmt.Sprintf("Content-Type: %s", contentType.String()))
	}

	if responseBody != "" {
		headers = append(headers, fmt.Sprintf("Content-Length: %d", len(responseBody)))
	}

	// 응답 구성
	response := status + "\r\n"
	if len(headers) > 0 {
		response += strings.Join(headers, "\r\n") + "\r\n"
	}
	response += "\r\n"

	if responseBody != "" {
		response += responseBody
	}

	return response
}

// routeRequest는 경로에 따라 적절한 HTTP 응답을 반환합니다
func routeRequest(request string) string {
	method := parseRequestMethod(request)
	path := parseRequestPath(request)
	headers := parseHeaders(request)

	// gzip 지원 여부 확인
	useGzip := supportsGzip(headers)

	switch {
	case path == "/":
		return handleResponse(200, ContentTypeNone, "", false)
	case strings.HasPrefix(path, "/echo/"):
		str := path[6:]
		return handleResponse(200, ContentTypeTextPlain, str, useGzip)
	case strings.HasPrefix(path, "/user-agent"):
		userAgent := headers["user-agent"]
		return handleResponse(200, ContentTypeTextPlain, userAgent, useGzip)
	case strings.HasPrefix(path, "/files"):
		filename := path[7:]
		return handleFileRequest(filename, method, request, useGzip)
	default:
		return handleResponse(404, ContentTypeNone, "", false)
	}
}

func handleFileRequest(filename string, method string, request string, useGzip bool) string {
	if method == "GET" {
		return handleFileGet(filename, useGzip)
	} else if method == "POST" {
		body := parseRequestBody(request)
		return handleFilePost(filename, body)
	}
	return handleResponse(404, ContentTypeNone, "", false)
}

func handleFileGet(filename string, useGzip bool) string {
	if filesDirectory == "" {
		return handleResponse(404, ContentTypeNone, "", false)
	}

	filePath := filepath.Join(filesDirectory, filename)

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return handleResponse(404, ContentTypeNone, "", false)
	}

	// 파일 응답은 gzip을 적용하지 않음 (바이너리 파일일 수 있으므로)
	return handleResponse(200, ContentTypeApplicationOctetStream, string(fileContent), false)
}

func handleFilePost(filename string, body string) string {
	if filesDirectory == "" {
		return handleResponse(404, ContentTypeNone, "", false)
	}

	filePath := filepath.Join(filesDirectory, filename)

	err := os.WriteFile(filePath, []byte(body), 0644)
	if err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		return handleResponse(500, ContentTypeNone, "", false)
	}
	return handleResponse(201, ContentTypeNone, "", false)
}
