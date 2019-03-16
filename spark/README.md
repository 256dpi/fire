# spark

Package spark implements a simple pub/sub mechanism that allows clients to watch resources.

To watch resources, the client initiates a WebSocket connection to the a group action and subscribes to streams:

```json
{
  "subscribe": {
    "items": {
      "state": true
    } 
  }
}
```

The server then stores the subscription under the defined key and forwards matching operations to the client:

```json
{
  "items": {
    "5c880eb87b0a67df9a6a2efc": "created"
  } 
}
```

If necessary, the client can unsubscribe using the specified keys at any time:

```json
{
  "unsubscribe": ["items"]
}
```
