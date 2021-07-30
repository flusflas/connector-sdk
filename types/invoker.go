// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package types

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/pkg/errors"
)

// Invoker is used to send requests to functions. Responses are
// returned via the Responses channel.
type Invoker struct {
	PrintResponse bool
	Client        *http.Client
	GatewayURL    string
	ContentType   string
	CallbackURL   string
	SendTopic     bool
	Responses     chan InvokerResponse
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
}

// NewInvoker constructs an Invoker instance
func NewInvoker(gatewayURL string, client *http.Client, contentType, callbackURL string, printResponse, sendTopic bool) *Invoker {
	return &Invoker{
		PrintResponse: printResponse,
		Client:        client,
		GatewayURL:    gatewayURL,
		ContentType:   contentType,
		CallbackURL:   callbackURL,
		SendTopic:     sendTopic,
		Responses:     make(chan InvokerResponse),
	}
}

// Invoke triggers a function by accessing the API Gateway
func (i *Invoker) Invoke(topicMap *TopicMap, topic string, message *[]byte) {
	i.InvokeWithContext(context.Background(), topicMap, topic, message)
}

//InvokeWithContext triggers a function by accessing the API Gateway while propagating context
func (i *Invoker) InvokeWithContext(ctx context.Context, topicMap *TopicMap, topic string, message *[]byte) {
	if len(*message) == 0 {
		i.Responses <- InvokerResponse{
			Context: ctx,
			Error:   fmt.Errorf("no message to send"),
		}
	}

	matchedFunctions := topicMap.Match(topic)
	for _, matchedFunction := range matchedFunctions {
		log.Printf("Invoke function: %s", matchedFunction)

		gwURL := fmt.Sprintf("%s/%s", i.GatewayURL, matchedFunction)
		reader := bytes.NewReader(*message)

		body, statusCode, header, doErr := invokeFunction(ctx, i.Client, gwURL, reader, i.getHeaders(topic))

		if doErr != nil {
			i.Responses <- InvokerResponse{
				Context: ctx,
				Error:   errors.Wrap(doErr, fmt.Sprintf("unable to invoke %s", matchedFunction)),
			}
			continue
		}

		i.Responses <- InvokerResponse{
			Context:  ctx,
			Body:     body,
			Status:   statusCode,
			Header:   header,
			Function: matchedFunction,
			Topic:    topic,
		}
	}
}

// getHeaders returns a map of headers to be included in the invocation request.
func (i *Invoker) getHeaders(topic string) (headers map[string]string) {
	if i.SendTopic && topic != "" {
		headers["X-Topic"] = topic
	}
	if i.CallbackURL != "" {
		headers["X-Callback-Url"] = i.CallbackURL
	}
	if i.ContentType != "" {
		headers["Content-Type"] = i.ContentType
	}
	return headers
}

func invokeFunction(ctx context.Context, c *http.Client, gwURL string, reader io.Reader, headers map[string]string) (*[]byte, int, *http.Header, error) {

	httpReq, err := http.NewRequest(http.MethodPost, gwURL, reader)
	if err != nil {
		return nil, http.StatusServiceUnavailable, nil, err
	}
	httpReq = httpReq.WithContext(ctx)

	if httpReq.Body != nil {
		defer httpReq.Body.Close()
	}

	for k, v := range headers {
		httpReq.Header.Add(k, v)
	}

	var body *[]byte

	res, doErr := c.Do(httpReq)
	if doErr != nil {
		return nil, http.StatusServiceUnavailable, nil, doErr
	}

	if res.Body != nil {
		defer res.Body.Close()

		bytesOut, readErr := ioutil.ReadAll(res.Body)
		if readErr != nil {
			log.Printf("Error reading body")
			return nil, http.StatusServiceUnavailable, nil, doErr

		}
		body = &bytesOut
	}

	return body, res.StatusCode, &res.Header, doErr
}
