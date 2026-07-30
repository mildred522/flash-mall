package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestOrderCycleUsesAuthenticatedCreateAndCancel(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/auth/login":
			_, _ = w.Write([]byte(`{"access_token":"token","user_id":9}`))
		case "/api/order/create":
			if r.Header.Get("Authorization") != "Bearer token" {
				http.Error(w, "missing bearer", http.StatusUnauthorized)
				return
			}
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if !strings.HasPrefix(body["request_id"].(string), "capacity-") {
				http.Error(w, "bad request id", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"order_id":"order-1","status":0}}`))
		case "/api/order/cancel":
			if r.Header.Get("Authorization") != "Bearer token" {
				http.Error(w, "missing bearer", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"order_id":"order-1","status":"closed"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newBusinessClient(server.URL, "13800000001", "password", 100)
	if err := client.login(t.Context()); err != nil {
		t.Fatal(err)
	}
	result := client.execute(t.Context(), "order-cycle", 1)
	if result.Err != nil || result.StatusCode != http.StatusOK {
		t.Fatalf("result=%+v", result)
	}

	mu.Lock()
	defer mu.Unlock()
	if strings.Join(paths, ",") != "/api/auth/login,/api/order/create,/api/order/cancel" {
		t.Fatalf("paths=%v", paths)
	}
}

func TestReadMixUsesOnlyBoundedPublicRoutes(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"data":{"items":[]}}`))
	}))
	defer server.Close()

	client := newBusinessClient(server.URL, "", "", 100)
	for sequence, want := range map[int64]string{
		0: "/api/shop/catalog",
		7: "/api/shop/products/detail",
		9: "/api/shop/stores/detail",
	} {
		result := client.execute(t.Context(), "read", sequence)
		if result.Err != nil || gotPath != want {
			t.Fatalf("sequence=%d path=%q want=%q err=%v", sequence, gotPath, want, result.Err)
		}
	}
}

func TestIdempotencyDoesNotDecodeAnUnusedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/auth/login":
			_, _ = w.Write([]byte(`{"access_token":"token","user_id":9}`))
		case "/api/order/create":
			_, _ = w.Write([]byte(`response body intentionally unused`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newBusinessClient(server.URL, "13800000001", "password", 100)
	if err := client.login(t.Context()); err != nil {
		t.Fatal(err)
	}
	result := client.execute(t.Context(), "idempotency", 1)
	if result.Err != nil {
		t.Fatalf("unused response must not be decoded: %v", result.Err)
	}
}

func TestOrderCycleDoesNotReuseRequestIDAcrossLoadPhases(t *testing.T) {
	var mu sync.Mutex
	var requestIDs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/auth/login":
			_, _ = w.Write([]byte(`{"access_token":"token","user_id":9}`))
		case "/api/order/create":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			requestIDs = append(requestIDs, body["request_id"].(string))
			orderID := len(requestIDs)
			mu.Unlock()
			_, _ = w.Write([]byte(`{"data":{"order_id":"order-` + fmt.Sprint(orderID) + `","status":0}}`))
		case "/api/order/cancel":
			_, _ = w.Write([]byte(`{"data":{"status":"closed"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newBusinessClient(server.URL, "13800000001", "password", 100)
	if err := client.login(t.Context()); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		result := client.execute(t.Context(), "order-cycle", 0)
		if result.Err != nil {
			t.Fatalf("order cycle failed: %v", result.Err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requestIDs) != 2 || requestIDs[0] == requestIDs[1] {
		t.Fatalf("request IDs must remain unique across warmup and measurement: %v", requestIDs)
	}
}
