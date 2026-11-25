package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serveReveal 向 /api/reveal 发起请求并返回响应。
func serveReveal(t *testing.T, app *App, body string, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/reveal", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	return rr
}

func TestRevealRejectsNonPost(t *testing.T) {
	app, _ := newTestApp(t)
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/reveal", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		rr := httptest.NewRecorder()
		app.routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s status = %d, want 405", method, rr.Code)
		}
	}
}

// 定位会在宿主机弹出文件管理器，因此远程请求必须一律拒绝。
func TestRevealRejectsRemoteRequest(t *testing.T) {
	app, root := newTestApp(t)
	writeTestFile(t, root, "技术类/文章.md", "# 标题")
	mustReload(t, app)

	rr := serveReveal(t, app, `{"path":"技术类/文章.md"}`, "192.0.2.7:5555")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("远程请求 status = %d, want 403", rr.Code)
	}
}

func TestRevealRejectsUnsafePath(t *testing.T) {
	app, root := newTestApp(t)
	writeTestFile(t, root, "技术类/文章.md", "# 标题")
	mustReload(t, app)

	cases := []string{
		`{"path":"../../../etc/passwd"}`,
		`{"path":"..\\..\\windows\\win.ini"}`,
		`{"path":"技术类/../../../secret.md"}`,
	}
	for _, body := range cases {
		rr := serveReveal(t, app, body, "127.0.0.1:1234")
		if rr.Code < 400 || rr.Code >= 500 {
			t.Errorf("body=%s status = %d, want 4xx", body, rr.Code)
		}
	}
}

func TestRevealRejectsMissingPath(t *testing.T) {
	app, _ := newTestApp(t)
	for _, body := range []string{`{}`, `{"path":""}`} {
		rr := serveReveal(t, app, body, "127.0.0.1:1234")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("body=%s status = %d, want 400", body, rr.Code)
		}
	}
}

func TestRevealRejectsUnknownFile(t *testing.T) {
	app, _ := newTestApp(t)
	rr := serveReveal(t, app, `{"path":"nope.md"}`, "127.0.0.1:1234")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestRevealRejectsMalformedJSON(t *testing.T) {
	app, _ := newTestApp(t)
	rr := serveReveal(t, app, `{"path":`, "127.0.0.1:1234")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestRevealAcceptsLoopback(t *testing.T) {
	app, root := newTestApp(t)
	writeTestFile(t, root, "技术类/文章.md", "# 标题")
	mustReload(t, app)

	// 替换真实调用，避免测试期间弹出系统窗口。
	var called string
	orig := revealFn
	revealFn = func(path string) error { called = path; return nil }
	t.Cleanup(func() { revealFn = orig })

	rr := serveReveal(t, app, `{"path":"技术类/文章.md"}`, "127.0.0.1:1234")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200，body=%s", rr.Code, rr.Body.String())
	}
	if called == "" {
		t.Fatal("未调用文件管理器定位")
	}
	if !strings.HasSuffix(called, "文章.md") {
		t.Fatalf("定位路径 = %q", called)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}
}

// Session 0 内启动的 explorer 窗口用户永远看不到，必须如实报错而不是假装成功。
func TestRevealFailsLoudlyInSessionZero(t *testing.T) {
	app, root := newTestApp(t)
	writeTestFile(t, root, "技术类/文章.md", "# 标题")
	mustReload(t, app)

	orig := revealFn
	revealFn = func(path string) error {
		return fmt.Errorf("无法在桌面会话打开（服务运行于 Session 0）：%w", errNoActiveSession)
	}
	t.Cleanup(func() { revealFn = orig })

	rr := serveReveal(t, app, `{"path":"技术类/文章.md"}`, "127.0.0.1:1234")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应应为 JSON: %v (body=%s)", err, rr.Body.String())
	}
	if body["error"] == "" {
		t.Fatal("缺少 error 字段")
	}
	// detail 必须透出，否则用户只看到"点了没反应"
	if !strings.Contains(body["detail"], "Session 0") {
		t.Fatalf("detail = %q, want 包含失败原因", body["detail"])
	}
}
