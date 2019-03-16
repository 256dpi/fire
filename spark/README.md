# spark

Package spark implements a simple pub/sub mechanism that allows clients to watch resources.

## WebSocket

To watch resources, the client initiates a WebSocket connection to the a group action:

```
wss://example.com/v1/api/watch
```
 
And then subscribes to streams:

```json
{
  "subscribe": {
    "items": {
      "state": true
    } 
  }
}
```

The server then forwards matching events to the client:

```json
{
  "items": {
    "5c880eb87b0a67df9a6a2efc": "created"
  } 
}
```

If necessary, the client can unsubscribe from streams:

```json
{
  "unsubscribe": ["items"]
}
```

## Server Sent Events

Alternatively, the client may use SSE to issue a single subscription:

```
https://example.com/v1/api/watch?s=items&d=eyAic3RhdGUiOiB0cnVlIH0=
```

- The `s` parameter is the name of the stream.
- The `d` parameter is the data as base64 encoded JSON.

The server will then forward events:

```
data: { "items": { "5c880eb87b0a67df9a6a2efc": "created" } }
```
