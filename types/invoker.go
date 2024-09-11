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
	opts          *InvokerOptions
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
func NewInvoker(gatewayURL string, client *http.Client, contentType string, printResponse, printRequest bool, opts ...InvokerOptionFunc) *Invoker {
	o := &InvokerOptions{}
	for _, optFn := range opts {
		optFn(o)
	}

	return &Invoker{
		PrintResponse: printResponse,
		PrintRequest:  printRequest,
		Client:        client,
		GatewayURL:    gatewayURL,
		ContentType:   contentType,
		Responses:     make(chan InvokerResponse, 1),
		opts:          o,
	}
}

// Invoke triggers the given function by accessing the API Gateway.
// functionName must include the namespace (e.g. "my-function.my-namespace").
func (i *Invoker) Invoke(ctx context.Context, functionName string, message *[]byte, headers http.Header, opts ...InvokeOptionFunc) InvokerResponse {
	o := &InvokeOptions{}
	for _, optFn := range opts {
		optFn(o)
	}

	log.Printf("Invoke function: %s", functionName)

	gwURL := fmt.Sprintf("%s/%s", i.GatewayURL, functionName)

	var reader *bytes.Reader
	if message == nil {
		reader = bytes.NewReader([]byte{})
	} else {
		reader = bytes.NewReader(*message)
	}

	topic := o.topic
	if i.PrintRequest {
		log.Printf("[connector] %s => %s body:\t%s", topic, functionName, string(*message))
	}

	if headers == nil {
		headers = http.Header{}
	}

	if i.opts.userAgent != "" {
		headers.Set("User-Agent", i.opts.userAgent)
	}

	if i.opts.sendTopic {
		headers.Set("X-Topic", topic)
	}

	if o.asyncCallbackURL != "" {
		headers.Set("X-Callback-Url", o.asyncCallbackURL)
	} else if i.opts.asyncCallbackURL != "" {
		headers.Set("X-Callback-Url", i.opts.asyncCallbackURL)
	}

	if o.contentType != "" {
		headers.Set("Content-Type", o.contentType)
	} else if i.ContentType != "" {
		headers.Set("Content-Type", i.ContentType)
	}

	start := time.Now()
	body, statusCode, header, err := i.invoke(ctx, i.Client, gwURL, reader, headers)
	if err != nil {
		invokerResponse := InvokerResponse{
			Context:  ctx,
			Status:   statusCode,
			Error:    fmt.Errorf("unable to invoke %s, error: %w", functionName, err),
			Function: functionName,
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

// InvokeWithTopic triggers a function using a topic by accessing the API Gateway.
func (i *Invoker) InvokeWithTopic(ctx context.Context, topicMap *TopicMap, topic string, message *[]byte, headers http.Header, opts ...InvokeOptionFunc) {
	if len(*message) == 0 || topicMap == nil {
		i.Responses <- InvokerResponse{
			Context:  ctx,
			Error:    fmt.Errorf("no message to send"),
			Duration: time.Millisecond * 0,
		}
		return
	}

	o := &InvokeOptions{}
	for _, optFn := range opts {
		optFn(o)
	}

	opts = append(opts, WithInvokeTopic(topic))

	matchedFunctions := topicMap.Match(topic)
	for _, matchedFunction := range matchedFunctions {
		i.Invoke(ctx, matchedFunction, message, headers, opts...)
	}
}

func (i *Invoker) invoke(ctx context.Context, c *http.Client, gwURL string, reader io.Reader, headers http.Header) (*[]byte, int, *http.Header, error) {
	req, err := http.NewRequest(http.MethodPost, gwURL, reader)
	if err != nil {
		return nil, http.StatusServiceUnavailable, nil, err
	}

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
