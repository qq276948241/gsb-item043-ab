package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/longbridgeapp/opencc"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	converter, err := opencc.New("t2s")
	if err != nil {
		t.Fatalf("opencc.New: %v", err)
	}
	return makeHandler(converter)
}

func doRequest(handler http.Handler, method, target, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestConvertExamples(t *testing.T) {
	handler := newTestHandler(t)
	cases := []struct{ in, want string }{
		{"學而時習之，不亦說乎？", "学而时习之，不亦说乎？"},
		{"學而時習之，不亦說乎？有朋自遠方來，不亦樂乎？", "学而时习之，不亦说乎？有朋自远方来，不亦乐乎？"},
		{"子曰：「學而時習之，不亦說乎？」", "子曰：「学而时习之，不亦说乎？」"},
		{"己所不欲，勿施於人。", "己所不欲，勿施于人。"},
		{"髮", "发"},
		{"發", "发"},
		{"乾", "干"},
		{"乾坤", "乾坤"},
		{"沈", "沈"},
		{"沈默", "沉默"},
		{"徵", "征"},
		{"宮商角徵羽", "宫商角徵羽"},
		{"著書", "著书"},
		{"【註】", "【注】"},
		{"《論語》", "《论语》"},
		{"臺灣", "台湾"},
		{"長安", "长安"},
		{"製作", "制作"},
		{"並非", "并非"},
		{"学而时习之，不亦说乎？", "学而时习之，不亦说乎？"},
		{"123，abc。", "123，abc。"},
		{" 學 ", " 学 "},
		{"\uFEFF學", "\uFEFF学"},
	}
	for _, c := range cases {
		recorder := doRequest(handler, http.MethodPost, "/zhuan", "text/plain; charset=utf-8", c.in)
		if recorder.Code != http.StatusOK {
			t.Errorf("%q: status = %d, want 200", c.in, recorder.Code)
			continue
		}
		if got := recorder.Body.String(); got != c.want {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
		}
		if ct := recorder.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
			t.Errorf("%q: Content-Type = %q", c.in, ct)
		}
	}
}

func TestIdempotent(t *testing.T) {
	handler := newTestHandler(t)
	body := "學而時習之，不亦說乎？有朋自遠方來，不亦樂乎？"
	first := doRequest(handler, http.MethodPost, "/zhuan", "text/plain", body).Body.String()
	second := doRequest(handler, http.MethodPost, "/zhuan", "text/plain", body).Body.String()
	if first != second {
		t.Errorf("两次结果不同：%q vs %q", first, second)
	}
}

func TestBadPathOrMethod(t *testing.T) {
	handler := newTestHandler(t)
	targets := []struct{ method, target string }{
		{http.MethodGet, "/zhuan"},
		{http.MethodPut, "/zhuan"},
		{http.MethodPost, "/"},
		{http.MethodPost, "/Zhuan"},
		{http.MethodPost, "/zhuan/"},
		{http.MethodPost, "/zhuan/extra"},
		{http.MethodPost, "/zhuan?a=1"},
		{http.MethodPost, "/zhuan?"},
	}
	for _, c := range targets {
		recorder := doRequest(handler, c.method, c.target, "text/plain; charset=utf-8", "學")
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("%s %s: status = %d, want 400", c.method, c.target, recorder.Code)
			continue
		}
		if got := recorder.Body.String(); got != "没法转换：请按说明送正文\n" {
			t.Errorf("%s %s: body = %q", c.method, c.target, got)
		}
	}
}

func TestBadContentType(t *testing.T) {
	handler := newTestHandler(t)
	types := []string{
		"",
		"application/json",
		"text/plain; charset=gbk",
		"text/plain; charset=utf-8; boundary=x",
		"text/plain; foo=bar",
		"text/html; charset=utf-8",
	}
	for _, ct := range types {
		recorder := doRequest(handler, http.MethodPost, "/zhuan", ct, "學")
		if recorder.Code != http.StatusBadRequest ||
			recorder.Body.String() != "没法转换：请送一段文字\n" {
			t.Errorf("Content-Type %q: status=%d body=%q", ct, recorder.Code, recorder.Body.String())
		}
	}
	recorder := doRequest(handler, http.MethodPost, "/zhuan", "application/json", "")
	if recorder.Body.String() != "没法转换：请送一段文字\n" {
		t.Errorf("空正文+坏类型: body = %q", recorder.Body.String())
	}
}

func TestGoodContentTypeVariants(t *testing.T) {
	handler := newTestHandler(t)
	types := []string{
		"text/plain",
		"text/plain; charset=utf-8",
		"text/plain; charset=UTF-8",
		"Text/Plain; Charset=utf-8",
		`text/plain; charset="utf-8"`,
		"text/plain;charset=utf-8",
	}
	for _, ct := range types {
		recorder := doRequest(handler, http.MethodPost, "/zhuan", ct, "學")
		if recorder.Code != http.StatusOK || recorder.Body.String() != "学" {
			t.Errorf("Content-Type %q: status=%d body=%q", ct, recorder.Code, recorder.Body.String())
		}
	}
}

func TestInvalidUTF8(t *testing.T) {
	handler := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/zhuan", strings.NewReader("\xff\xfe"))
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest ||
		recorder.Body.String() != "没法转换：请送一段文字\n" {
		t.Errorf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestEmptyBody(t *testing.T) {
	handler := newTestHandler(t)
	for _, body := range []string{"", " ", "\t\n\r ", "　", " 　 \t"} {
		recorder := doRequest(handler, http.MethodPost, "/zhuan", "text/plain; charset=utf-8", body)
		if recorder.Code != http.StatusBadRequest ||
			recorder.Body.String() != "没法转换：正文是空的\n" {
			t.Errorf("body %q: status=%d body=%q", body, recorder.Code, recorder.Body.String())
		}
	}
}
