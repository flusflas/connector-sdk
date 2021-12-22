// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package types

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Invoker is used to send requests to functions. Responses are
// returned via the Responses channel.
type Invoker struct {
	PrintResponse bool
	PrintRequest  bool
	Client        *http.Client
	GatewayURL    string
	ContentType   string
	Responses     chan InvokerResponse
	UserAgent     string
	CallbackURL   string
	SendTopic     bool
}

// InvokerResponse is a wrapper to contain the response or error the Invoker
// receives from the function. Networking errors wil be found in the Error field.
type InvokerResponse struct {
	Context  context.Context
	Body     *[]byte
	Header   *http.Header
	Status   int
	Error    error
	Topic    string
	Function string
	Duration time.Duration
}

// NewInvoker constructs an Invoker instance
func NewInvoker(gatewayURL string, client *http.Client, contentType string, printResponse, printRequest bool, userAgent string, callbackURL string, sendTopic bool) *Invoker {
	return &Invoker{
		PrintResponse: printResponse,
		PrintRequest:  printRequest,
		Client:        client,
		GatewayURL:    gatewayURL,
		ContentType:   contentType,
		Responses:     make(chan InvokerResponse),
		CallbackURL:   callbackURL,
		SendTopic:     sendTopic,
	}
}

// InvokeFunction triggers the given function by accessing the API Gateway.
// functionName must include the namespace (e.g. "my-function.my-namespace").
func (i *Invoker) InvokeFunction(functionName string, message *[]byte, headers http.Header) InvokerResponse {
	return i.InvokeFunctionWithContext(context.Background(), functionName, message, headers)
}

// InvokeFunctionWithContext triggers the given function by accessing the
// API Gateway while propagating context.
// functionName must include the namespace (e.g. "my-function.my-namespace").
func (i *Invoker) InvokeFunctionWithContext(ctx context.Context, functionName string, message *[]byte, headers http.Header) InvokerResponse {
	var reader *bytes.Reader

	log.Printf("Invoke function: %s", functionName)

	gwURL := fmt.Sprintf("%s/%s", i.GatewayURL, functionName)

	if message == nil {
		reader = bytes.NewReader([]byte{})
	} else {
		reader = bytes.NewReader(*message)
	}

	topic := headers.Get("X-Topic")
	if i.PrintRequest {
		log.Printf("[connector] %s => %s body:\t%s", topic, functionName, string(*message))
	}

	if headers == nil {
		headers = http.Header{}
	}
	if i.CallbackURL != "" {
		headers.Set("X-Callback-Url", i.CallbackURL)
	}
	if i.ContentType != "" {
		headers.Set("Content-Type", i.ContentType)
	}

	start := time.Now()
	body, statusCode, header, err := i.invoke(ctx, i.Client, gwURL, reader, headers)
	if err != nil {
		invokerResponse := InvokerResponse{
			Context:  ctx,
			Error:    fmt.Errorf("unable to invoke %s, error: %w", functionName, err),
			Duration: time.Since(start),
		}
		i.Responses <- invokerResponse
		return invokerResponse
	}

	invokerResponse := InvokerResponse{
		Context:  ctx,
		Body:     body,
		Status:   statusCode,
		Header:   header,
		Function: functionName,
		Topic:    topic,
		Duration: time.Since(start),
	}
	i.Responses <- invokerResponse
	return invokerResponse
}

// Invoke triggers a function using a topic by accessing the API Gateway.
func (i *Invoker) Invoke(topicMap *TopicMap, topic string, message *[]byte, headers http.Header) {
	i.InvokeWithContext(context.Background(), topicMap, topic, message, headers)
}

// InvokeWithContext triggers a function using a topic by accessing the API Gateway while propagating context.
func (i *Invoker) InvokeWithContext(ctx context.Context, topicMap *TopicMap, topic string, message *[]byte, headers http.Header) {
	if len(*message) == 0 {
		i.Responses <- InvokerResponse{
			Context:  ctx,
			Error:    fmt.Errorf("no message to send"),
			Duration: time.Millisecond * 0,
		}
	}

	matchedFunctions := topicMap.Match(topic)
	for _, matchedFunction := range matchedFunctions {
		if headers == nil {
			headers = http.Header{}
		}
		if i.SendTopic {
			headers.Set("X-Topic", topic)
		}
		i.InvokeFunctionWithContext(ctx, matchedFunction, message, headers)
	}
}

func (i *Invoker) invoke(ctx context.Context, c *http.Client, gwURL string, reader io.Reader, headers http.Header) (*[]byte, int, *http.Header, error) {
	req, err := http.NewRequest(http.MethodPost, gwURL, reader)
	if err != nil {
		return nil, http.StatusServiceUnavailable, nil, err
	}

	req.Header.Set("User-Agent", i.UserAgent)

	if v := req.Header.Get("X-Connector"); v == "" {
		req.Header.Set("X-Connector", "connector-sdk")
	}

	for k, values := range headers {
		for _, value := range values {
			req.Header.Add(k, value)
		}
	}

	req = req.WithContext(ctx)
	if req.Body != nil {
		defer req.Body.Close()
	}

	var body *[]byte
	res, err := c.Do(req)
	if err != nil {
		return nil, http.StatusServiceUnavailable, nil,
			fmt.Errorf("unable to reach endpoint %s, error: %w", gwURL, err)
	}

	if res.Body != nil {
		defer res.Body.Close()

		bytesOut, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, http.StatusServiceUnavailable,
				nil,
				fmt.Errorf("unable to read body from response %w", err)
		}
		body = &bytesOut
	}

	return body, res.StatusCode, &res.Header, err
}
