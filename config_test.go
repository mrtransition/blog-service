package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

// 根目录是宿主机绝对路径，只应返回给本机请求；绑定局域网时不能泄露给远程用户。
func TestConfigHidesRootDirFromRemote(t *testing.T) {
	app, _ := newTestApp(t)

	loopback := serve(t, app, http.MethodGet, "/api/config", func(r *http.Request) {
		r.RemoteAddr = "127.0.0.1:1234"
	})
	if loopback.Code != http.StatusOK {
		t.Fatalf("本机请求 status = %d, want 200", loopback.Code)
	}
	var local map[string]string
	if err := json.Unmarshal(loopback.Body.Bytes(), &local); err != nil {
		t.Fatalf("json: %v", err)
	}
	if local["rootDir"] == "" {
		t.Fatal("本机请求应返回 rootDir")
	}
	if local["blogName"] == "" {
		t.Fatal("应始终返回 blogName")
	}

	remote := serve(t, app, http.MethodGet, "/api/config", func(r *http.Request) {
		r.RemoteAddr = "192.0.2.9:5555"
	})
	if remote.Code != http.StatusOK {
		t.Fatalf("远程请求 status = %d, want 200", remote.Code)
	}
	var outside map[string]string
	if err := json.Unmarshal(remote.Body.Bytes(), &outside); err != nil {
		t.Fatalf("json: %v", err)
	}
	if _, ok := outside["rootDir"]; ok {
		t.Fatalf("远程请求不应返回 rootDir，实际 body = %v", outside)
	}
	if outside["blogName"] == "" {
		t.Fatal("远程请求仍应返回 blogName")
	}
}
