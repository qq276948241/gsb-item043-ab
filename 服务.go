package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/longbridgeapp/opencc"
)

const (
	errBody     = "没法转换：请按说明送正文\n"
	errType     = "没法转换：请送一段文字\n"
	errEmpty    = "没法转换：正文是空的\n"
	errNoLib    = "没法转换：转换库还没装好\n"
	errPortBusy = "没法转换：口子被占了\n"
)

func validContentType(header string) bool {
	if header == "" {
		return false
	}
	parts := strings.Split(header, ";")
	if strings.TrimSpace(parts[0]) == "" ||
		!strings.EqualFold(strings.TrimSpace(parts[0]), "text/plain") {
		return false
	}
	params := parts[1:]
	if len(params) > 1 {
		return false
	}
	if len(params) == 1 {
		kv := strings.SplitN(params[0], "=", 2)
		if len(kv) != 2 || !strings.EqualFold(strings.TrimSpace(kv[0]), "charset") {
			return false
		}
		value := strings.TrimSpace(kv[1])
		value = strings.Trim(value, `"'`)
		if !strings.EqualFold(value, "utf-8") {
			return false
		}
	}
	return true
}

func fail(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	io.WriteString(w, msg)
}

func main() {
	converter, err := opencc.New("t2s")
	if err != nil {
		fmt.Fprint(os.Stderr, errNoLib)
		os.Exit(2)
	}
	fmt.Println("库已载入")

	handler := makeHandler(converter)

	listener, err := net.Listen("tcp", "127.0.0.1:1843")
	if err != nil {
		fmt.Fprint(os.Stderr, errPortBusy)
		os.Exit(2)
	}
	http.Serve(listener, handler)
}

func makeHandler(converter *opencc.OpenCC) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/zhuan" ||
			r.URL.RawQuery != "" || r.URL.ForceQuery {
			fail(w, errBody)
			return
		}
		if !validContentType(r.Header.Get("Content-Type")) {
			fail(w, errType)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || !utf8.Valid(body) {
			fail(w, errType)
			return
		}
		text := string(body)
		if strings.Trim(text, " \t\n\r　") == "" {
			fail(w, errEmpty)
			return
		}
		converted, err := converter.Convert(text)
		if err != nil {
			fail(w, errType)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, converted)
	})
}
