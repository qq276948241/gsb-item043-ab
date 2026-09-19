package main

import (
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/longbridgeapp/opencc"
)

const (
	addr = "127.0.0.1:1843"

	msgBadRequest = "没法转换：请按说明送正文\n"
	msgNotText    = "没法转换：请送一段文字\n"
	msgEmpty      = "没法转换：正文是空的\n"
	msgNoLib      = "没法转换：转换库还没装好\n"
	msgPortBusy   = "没法转换：口子被占了\n"

	plainUTF8 = "text/plain; charset=utf-8"
)

var converter *opencc.OpenCC

func fail(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", plainUTF8)
	w.WriteHeader(http.StatusBadRequest)
	io.WriteString(w, msg)
}

func validContentType(header string) bool {
	if header == "" {
		return false
	}
	mediaType, params, err := mime.ParseMediaType(header)
	if err != nil || mediaType != "text/plain" {
		return false
	}
	for name, value := range params {
		if name != "charset" || !strings.EqualFold(value, "utf-8") {
			return false
		}
	}
	return true
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/zhuan" ||
		r.URL.RawQuery != "" || r.URL.ForceQuery {
		fail(w, msgBadRequest)
		return
	}

	if !validContentType(r.Header.Get("Content-Type")) {
		fail(w, msgNotText)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil || !utf8.Valid(body) {
		fail(w, msgNotText)
		return
	}

	text := string(body)
	if strings.Trim(text, " \t\n\r　") == "" {
		fail(w, msgEmpty)
		return
	}

	converted, err := converter.Convert(text)
	if err != nil {
		fail(w, msgNotText)
		return
	}

	w.Header().Set("Content-Type", plainUTF8)
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, converted)
}

func main() {
	cc, err := opencc.New("t2s")
	if err != nil {
		fmt.Fprint(os.Stderr, msgNoLib)
		os.Exit(2)
	}
	converter = cc

	fmt.Println("库已载入")

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprint(os.Stderr, msgPortBusy)
		os.Exit(2)
	}

	server := &http.Server{Handler: http.HandlerFunc(handler)}
	if err := server.Serve(listener); err != nil {
		fmt.Fprint(os.Stderr, msgPortBusy)
		os.Exit(2)
	}
}
