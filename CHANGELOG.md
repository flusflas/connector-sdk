# Changelog

## [0.2.0](https://github.com/flusflas/connector-sdk/tree/v0.2.0) (TBD)

The `Invoker` struct has been refactored to extend its usability through the use
of optional functions that can override the default behavior.

### Added

- The `Controller` interface and the `Invoker` struct now accept a variadic list
  of functional options to further customize their behavior:
  - `WithInvokeTopic()`: Invokes the functions that match the topic.
  - `WithInvokeAsyncCallbackURL()`: Sets the callback URL for asynchronous invocations.
  - `WithInvokeContentType()`: Sets the content type for the invoker.


- The `Invoker` constructor now accepts a variadic list of functional options:
  - `WithInvokerSendTopic()`: Whether to send the topic in the invocation request headers.
  - `WithInvokerAsyncCallbackURL()`: Sets the callback URL for asynchronous invocations.
  - `WithInvokerUserAgent()`: Sets the user agent for the invoker.

### Removed

- All the original `Invoke...()` methods have been removed from the `Invoker` struct.
  They have been replaced by the `Invoke()` and `InvokeWithContext()` methods.


## [0.1.0](https://github.com/flusflas/connector-sdk/tree/v0.1.0) (2021-07-30)

### Added

- **Namespace filtering**.
  This is useful in multi-tenant environments. If a namespace is set in the
  `ControllerConfig` instance, only the functions in that namespace will be
  mapped. Otherwise, all functions will be mapped (regardless of their
  namespace).
  ```go
  config := &types.ControllerConfig{
    ...
    Namespace:                namespace,
  }
  ```
- **Callback URL for asynchronous invocations**.
  A callback URL can be set using `AsyncFunctionCallbackURL`:
  ```go
  config := &types.ControllerConfig{
    ...
    AsyncFunctionInvocation:  true,
    AsyncFunctionCallbackURL: asyncCallbackURL,
  }
  ```
- **Custom topic matcher**.
  A custom function can be used for topic matching to override the default
  equality check between received topic and function topic.
  ```go
  matchLowercase := func(topic, route string) bool {
      return strings.ToLower(topic) == strings.ToLower(route)
  }
  
  config := &types.ControllerConfig{
    ...
    TopicMatcher: matchLowercase,
  }
  ```
- **Send message topic to function**.
  To give some context to the invoked functions, the topic can optionally be
  sent in the invocation requests in an `X-Topic` header.
  ```go
  config := &types.ControllerConfig{
    ...
    SendTopic: true,
  }
  ```
- **Invoke functions without a topic**.
  A topic may be not required in many applications. The `Invoker` includes two
  new methods to allow invoking a function using its name instead of a topic.
  ```go
  invoker.InvokeFunction("my-function.my.namespace", message, headers)
  invoker.InvokeFunctionWithContext(ctx, "my-function.my.namespace", message, headers)
  ```