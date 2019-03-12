# spark

Package spark implements a simple pub/sub mechanism that allows clients to watch resources.

## Watch Token

A watch token represents a JWT token that allows the client to watch resources on the server. The token is issued by the server and requested by the client via a the collection or resource actions mechanism.

In the data property of the token the server specifies selectors that are used by the server to check which records should be forwarded. Selectors for collection watches are multiple equality filters while selectors for resource watches are just the resource id.

A fully formatted collection watch token might have the following JTW claims:

```json
{
  "sub": "foos",
  "iat": 1516239022,
  "exp": 1516240022,
  "dat": {
    "tenant": "5c88178b7b0a67e45de16f57",
    "state": "active" 
  }
}
```

And a fully formatted resource watch token might look like the following:

```json
{
  "sub": "foos",
  "id": "5c88178b7b0a67e45de16f57",
  "iat": 1516239022,
  "exp": 1516240022
}
```

Collection watch tokens only match "insert" and "update" operations while resource watch tokens also match "delete" operations.

## Watch Endpoint

To watch resources, the client initiates a WebSocket connection to the a group action and subscribes using earlier obtained watch tokens:

```json
{
  "subscribe": {
    "1": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" 
  }
}
```

The server then stores the subscription under the defined key and forwards matching operations to the client:

```json
{
  "foos": {
    "5c880eb87b0a67df9a6a2efc": "insert"
  } 
}
```

If necessary, the client can unsubscribe using the specified keys at any time:

```json
{
  "unsubscribe": ["1"]
}
```
