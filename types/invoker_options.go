package types

// InvokerOptionFunc defines a function that modifies an InvokerOptions instance.
type InvokerOptionFunc func(*InvokerOptions)

// InvokerOptions defines additional configuration options for an Invoker.
type InvokerOptions struct {
	// sendTopic indicates whether the topic of the message used for invoking a
	// function has to be sent in the "X-Topic" header of the request.
	sendTopic bool
	// asyncCallbackURL is the callback URL used when invoking a function
	// asynchronously.
	asyncCallbackURL string
	// userAgent defines the user agent to be used in the invocation requests.
	userAgent string
}

// WithInvokerSendTopic sets whether the topic of the message used for invoking a
// function has to be sent in the "X-Topic" header of the request.
func WithInvokerSendTopic(sendTopic bool) InvokerOptionFunc {
	return func(opts *InvokerOptions) {
		opts.sendTopic = sendTopic
	}
}

// WithInvokerAsyncCallbackURL sets the callback URL used when invoking a
// function asynchronously.
func WithInvokerAsyncCallbackURL(callbackURL string) InvokerOptionFunc {
	return func(opts *InvokerOptions) {
		opts.asyncCallbackURL = callbackURL
	}
}

// WithInvokerUserAgent sets the user agent to be used in the invocation requests.
func WithInvokerUserAgent(userAgent string) InvokerOptionFunc {
	return func(opts *InvokerOptions) {
		opts.userAgent = userAgent
	}
}

// InvokeOptionFunc defines a function that modifies an InvokeOptions instance.
type InvokeOptionFunc func(*InvokeOptions)

// InvokeOptions defines additional configuration options for the invocation of
// a function using Invoker.Invoke.
type InvokeOptions struct {
	// topic is the topic of the message used for invoking a function.
	topic string
	// asyncCallbackURL is the callback URL used when invoking a function
	// asynchronously. It will override the callback URL set in the InvokerOptions.
	asyncCallbackURL string
	// contentType defines which content type will be set in the header of
	// invocation requests. It will override Invoker.ContentType.
	contentType string
}

// WithInvokeTopic sets the topic of the message used for invoking a function.
func WithInvokeTopic(topic string) InvokeOptionFunc {
	return func(opts *InvokeOptions) {
		opts.topic = topic
	}
}

// WithInvokeAsyncCallbackURL sets the callback URL used when invoking a function
// asynchronously. It will override the callback URL set in the InvokerOptions.
func WithInvokeAsyncCallbackURL(callbackURL string) InvokeOptionFunc {
	return func(opts *InvokeOptions) {
		opts.asyncCallbackURL = callbackURL
	}
}

// WithInvokeContentType sets the content type to be set in the header of
// invocation requests. It will override Invoker.ContentType.
func WithInvokeContentType(contentType string) InvokeOptionFunc {
	return func(opts *InvokeOptions) {
		opts.contentType = contentType
	}
}
