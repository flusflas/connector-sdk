package types

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func pointer[T any](v T) *T {
	return &v
}

func Test_Invoke(t *testing.T) {
	type args struct {
		ctx      context.Context
		function string
		message  *[]byte
		headers  http.Header
		opts     []InvokeOptionFunc
	}
	type want struct {
		resp        InvokerResponse
		handlerFunc http.HandlerFunc
	}

	tests := []struct {
		name        string
		invokerOpts []InvokerOptionFunc
		args        args
		contentType string
		want        want
	}{
		{
			name: "success",
			args: args{
				ctx:      context.Background(),
				function: "test-function",
				message:  pointer([]byte(`{"foo": "request"}`)),
				headers:  http.Header{},
				opts:     nil,
			},
			contentType: "application/json",
			want: want{
				resp: InvokerResponse{
					Context:  context.Background(),
					Error:    nil,
					Body:     pointer([]byte(`{"foo": "response"}`)),
					Status:   http.StatusOK,
					Header:   nil,
					Function: "test-function",
				},
				handlerFunc: func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("Expected content type application/json, got %s", r.Header.Get("Content-Type"))
					}
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Errorf("Error reading request body: %v", err)
					}
					if string(body) != `{"foo": "request"}` {
						t.Errorf("Expected body %q, got %s", `{"foo": "request"}`, string(body))
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"foo": "response"}`))
				},
			},
		},
		{
			name: "success with options",
			invokerOpts: []InvokerOptionFunc{
				WithInvokerSendTopic(true),
			},
			args: args{
				ctx:      context.Background(),
				function: "test-function",
				message:  pointer([]byte("text message")),
				headers:  http.Header{},
				opts: []InvokeOptionFunc{
					WithInvokeTopic("test-topic"),
					WithInvokeAsyncCallbackURL("http://callback-url"),
					WithInvokeContentType("plain/text"),
				},
			},
			contentType: "application/json",
			want: want{
				resp: InvokerResponse{
					Context:  context.Background(),
					Error:    nil,
					Body:     pointer([]byte(`{"foo": "response"}`)),
					Status:   http.StatusOK,
					Header:   nil,
					Function: "test-function",
					Topic:    "test-topic",
				},
				handlerFunc: func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Content-Type") != "plain/text" {
						t.Errorf("Expected content type plain/text, got %s", r.Header.Get("Content-Type"))
					}
					if r.Header.Get("X-Topic") != "test-topic" {
						t.Errorf("Expected topic test-topic, got %s", r.Header.Get("X-Topic"))
					}
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Errorf("Error reading request body: %v", err)
					}
					if string(body) != "text message" {
						t.Errorf("Expected body %q, got %s", "text message", string(body))
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"foo": "response"}`))
				},
			},
		},
		{
			name: "no message",
			args: args{
				ctx:      context.Background(),
				function: "test-function",
				message:  pointer([]byte("")),
				headers:  http.Header{},
			},
			contentType: "application/json",
			want: want{
				resp: InvokerResponse{
					Context:  context.Background(),
					Error:    fmt.Errorf("no message to send"),
					Function: "test-function",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.want.handlerFunc)
			defer srv.Close()

			client := srv.Client()
			invoker := NewInvoker(srv.URL, client, tt.contentType, false, false, tt.invokerOpts...)

			var chanResponses []InvokerResponse
			go func() {
				for {
					chanResponses = append(chanResponses, <-invoker.Responses)
				}
			}()

			resp := invoker.Invoke(tt.args.ctx, tt.args.function, tt.args.message, tt.args.headers, tt.args.opts...)

			// Wait for the goroutine to receive the response
			time.Sleep(1 * time.Millisecond)
			if len(chanResponses) != 1 {
				t.Errorf("Expected 1 response, got %d", len(chanResponses))
			}

			resp.Duration = chanResponses[0].Duration
			tt.want.resp.Duration = chanResponses[0].Duration
			if !compareInvokerResponse(t, chanResponses[0], tt.want.resp) {
				t.Errorf("Expected chanResp to be equal to resp")
			}
			if !compareInvokerResponse(t, resp, tt.want.resp) {
				t.Errorf("Expected resp to be equal to want")
			}
		})
	}
}

func Test_Invoke_NetworkError(t *testing.T) {
	client := &http.Client{}
	invoker := NewInvoker("http://invalid-url", client, "application/json", false, false)

	var chanResponses []InvokerResponse
	go func() {
		for {
			chanResponses = append(chanResponses, <-invoker.Responses)
		}
	}()

	ctx := context.Background()
	message := []byte("test message")
	headers := http.Header{}

	resp := invoker.Invoke(ctx, "test-function", &message, headers)

	if resp.Error == nil {
		t.Errorf("Expected error, got nil")
	}
	if resp.Status != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, resp.Status)
	}

	// Wait for the goroutine to receive the response
	time.Sleep(1 * time.Millisecond)
	if len(chanResponses) != 1 {
		t.Errorf("Expected 1 response, got %d", len(chanResponses))
	}

	expectedInvokerResponse := InvokerResponse{
		Context:  ctx,
		Status:   http.StatusServiceUnavailable,
		Error:    fmt.Errorf("unable to invoke test-function, error: unable to reach endpoint http://invalid-url/test-function, error: Post \"http://invalid-url/test-function\": dial tcp"),
		Function: "test-function",
		Duration: chanResponses[0].Duration,
	}
	if !compareInvokerResponse(t, chanResponses[0], expectedInvokerResponse) {
		t.Errorf("Expected chanResp to be equal to resp")
	}
}

func Test_InvokeWithTopic_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	}))
	defer srv.Close()

	client := srv.Client()
	invoker := NewInvoker(srv.URL, client, "application/json", false, false)

	var chanResponses []InvokerResponse
	go func() {
		for {
			chanResponses = append(chanResponses, <-invoker.Responses)
		}
	}()

	ctx := context.Background()
	message := []byte("test message")
	headers := http.Header{}
	headers.Set("Custom-Header", "value")

	topicMap := &TopicMap{
		lookup: &map[string][]string{
			"test-topic1": {"test-function1", "test-function2"},
			"test-topic2": {"test-function3"},
		},
		matchFunc: defaultMatchTopic,
	}

	invoker.InvokeWithTopic(ctx, topicMap, "test-topic1", &message, headers)

	// Wait for the goroutine to receive the response
	time.Sleep(1 * time.Millisecond)
	if len(chanResponses) != 2 {
		t.Errorf("Expected 1 response, got %d", len(chanResponses))
	}

	expectedInvokerResponse1 := InvokerResponse{
		Context:  ctx,
		Error:    nil,
		Body:     pointer([]byte("response")),
		Status:   http.StatusOK,
		Header:   nil,
		Function: "test-function1",
		Topic:    "test-topic1",
		Duration: chanResponses[0].Duration,
	}
	if !compareInvokerResponse(t, chanResponses[0], expectedInvokerResponse1) {
		t.Errorf("Expected chanResp to be equal to resp")
	}

	expectedInvokerResponse2 := InvokerResponse{
		Context:  ctx,
		Error:    nil,
		Body:     pointer([]byte("response")),
		Status:   http.StatusOK,
		Header:   nil,
		Function: "test-function2",
		Topic:    "test-topic1",
		Duration: chanResponses[1].Duration,
	}
	if !compareInvokerResponse(t, chanResponses[1], expectedInvokerResponse2) {
		t.Errorf("Expected chanResp to be equal to resp")
	}
}

func Test_InvokeWithTopic_NoMessage(t *testing.T) {
	client := &http.Client{}
	invoker := NewInvoker("http://localhost", client, "application/json", false, false)

	ctx := context.Background()
	message := []byte("")
	headers := http.Header{}

	topicMap := &TopicMap{
		lookup: &map[string][]string{
			"test-topic": {"test-function"},
		},
		//matchFunc: nil,
	}

	invoker.InvokeWithTopic(ctx, topicMap, "test-topic", &message, headers)

	select {
	case resp := <-invoker.Responses:
		if resp.Error == nil {
			t.Errorf("Expected error, got nil")
		}
		if resp.Status != 0 {
			t.Errorf("Expected status 0, got %d", resp.Status)
		}
	case <-time.After(1 * time.Second):
		t.Errorf("Expected response, got timeout")
	}
}

func compareInvokerResponse(t *testing.T, r1, r2 InvokerResponse) bool {
	t.Helper()

	if r1.Context != r2.Context {
		t.Errorf("Context mismatch: %v != %v", r1.Context, r2.Context)
		return false
	}
	if (r1.Body == nil) != (r2.Body == nil) {
		t.Errorf("Body mismatch: %v != %v", r1.Body, r2.Body)
		return true
	}
	if r1.Body != nil && r2.Body != nil && string(*r1.Body) != string(*r2.Body) {
		t.Errorf("Body mismatch: %s != %s", *r1.Body, *r2.Body)
		return false
	}
	if r1.Status != r2.Status {
		t.Errorf("Status mismatch: %d != %d", r1.Status, r2.Status)
		return false
	}
	if (r1.Error == nil) != (r2.Error == nil) {
		t.Errorf("Error mismatch: %v != %v", r1.Error, r2.Error)
		return false
	}
	if r1.Error != nil && r2.Error != nil && !strings.Contains(r1.Error.Error(), r2.Error.Error()) {
		t.Errorf("Error mismatch: %v != %v", r1.Error, r2.Error)
		return false
	}
	if r1.Topic != r2.Topic {
		t.Errorf("Topic mismatch: %s != %s", r1.Topic, r2.Topic)
		return false
	}
	if r1.Function != r2.Function {
		t.Errorf("Function mismatch: %s != %s", r1.Function, r2.Function)
		return false
	}
	if r1.Duration != r2.Duration {
		t.Errorf("Duration mismatch: %v != %v", r1.Duration, r2.Duration)
		return false
	}
	return true
}
