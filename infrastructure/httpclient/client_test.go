package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

/** New */

func TestNew_DefaultTimeout(t *testing.T) {
	client := New(Config{BaseURL: "http://localhost"})
	if client.httpClient.Timeout != 10*time.Second {
		t.Errorf("expected default timeout 10s, got %v", client.httpClient.Timeout)
	}
}

func TestNew_CustomTimeout(t *testing.T) {
	client := New(Config{BaseURL: "http://localhost", Timeout: 30 * time.Second})
	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", client.httpClient.Timeout)
	}
}

func TestNew_BaseURLPreserved(t *testing.T) {
	client := New(Config{BaseURL: "http://example.com"})
	if client.baseURL != "http://example.com" {
		t.Errorf("expected baseURL 'http://example.com', got %q", client.baseURL)
	}
}

func TestNew_APIKeyPreserved(t *testing.T) {
	client := New(Config{BaseURL: "http://localhost", APIKey: "test-key-123"})
	if client.apiKey != "test-key-123" {
		t.Errorf("expected apiKey 'test-key-123', got %q", client.apiKey)
	}
}

/** Get */

func TestGet_Success(t *testing.T) {
	type response struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/users" {
			t.Errorf("expected /api/users, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response{Name: "John", Age: 30})
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	var result response
	err := client.Get(context.Background(), "/api/users", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "John" {
		t.Errorf("expected Name 'John', got %q", result.Name)
	}
	if result.Age != 30 {
		t.Errorf("expected Age 30, got %d", result.Age)
	}
}

func TestGet_QueryParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids := r.URL.Query().Get("ids")
		if ids != "a,b,c" {
			t.Errorf("expected ids=a,b,c, got %q", ids)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	var result []string
	err := client.Get(context.Background(), "/api/items?ids=a,b,c", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGet_NonSuccessStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	var result map[string]string
	err := client.Get(context.Background(), "/api/notfound", &result)
	if err == nil {
		t.Error("expected error for 404 status")
	}
}

func TestGet_NilResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	err := client.Get(context.Background(), "/api/test", nil)
	if err != nil {
		t.Fatalf("unexpected error with nil result: %v", err)
	}
}

/** Post */

func TestPost_Success(t *testing.T) {
	type reqBody struct {
		Name string `json:"name"`
	}
	type resBody struct {
		ID string `json:"id"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", r.Header.Get("Content-Type"))
		}

		body, _ := io.ReadAll(r.Body)
		var req reqBody
		json.Unmarshal(body, &req)
		if req.Name != "Alice" {
			t.Errorf("expected Name 'Alice', got %q", req.Name)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resBody{ID: "abc-123"})
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	var result resBody
	err := client.Post(context.Background(), "/api/users", reqBody{Name: "Alice"}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "abc-123" {
		t.Errorf("expected ID 'abc-123', got %q", result.ID)
	}
}

func TestPost_NilBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if len(body) != 0 {
			t.Errorf("expected empty body, got %d bytes", len(body))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	var result map[string]interface{}
	err := client.Post(context.Background(), "/api/action", nil, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

/** Put */

func TestPut_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"updated":true}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	var result map[string]bool
	err := client.Put(context.Background(), "/api/users/1", map[string]string{"name": "Bob"}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result["updated"] {
		t.Error("expected updated=true")
	}
}

/** Delete */

func TestDelete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"deleted":true}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	var result map[string]bool
	err := client.Delete(context.Background(), "/api/users/1", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result["deleted"] {
		t.Error("expected deleted=true")
	}
}

/** API Key Header */

func TestAPIKeyHeader_Set(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "my-secret-key" {
			t.Errorf("expected X-API-Key 'my-secret-key', got %q", apiKey)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL, APIKey: "my-secret-key"})
	err := client.Get(context.Background(), "/api/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIKeyHeader_NotSet_WhenEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "" {
			t.Errorf("expected no X-API-Key header, got %q", apiKey)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	err := client.Get(context.Background(), "/api/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

/** Error Handling */

func TestServerError_500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`internal error`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	err := client.Get(context.Background(), "/api/fail", nil)
	if err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestInvalidJSON_Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	var result map[string]string
	err := client.Get(context.Background(), "/api/test", &result)
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := client.Get(ctx, "/api/slow", nil)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestUnmarshalBodyError_MarshalRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	// channels can't be marshaled to JSON
	err := client.Post(context.Background(), "/api/test", make(chan int), nil)
	if err == nil {
		t.Error("expected error for unmarshalable body")
	}
}

/** Status Code Boundaries */

func TestStatusCode_299_IsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(299)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	err := client.Get(context.Background(), "/api/test", nil)
	if err != nil {
		t.Errorf("299 should be success, got error: %v", err)
	}
}

func TestStatusCode_300_IsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(300)
		w.Write([]byte(`redirect`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	err := client.Get(context.Background(), "/api/test", nil)
	if err == nil {
		t.Error("300 should be error")
	}
}
