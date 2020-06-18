package nitro

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// Client is a reusable client for accessing procedure endpoints.
type Client struct {
	base string
	http http.Client
}

// NewClient will create and return a new client using the provided base
// address.
func NewClient(base string) *Client {
	return &Client{
		base: base,
	}
}

// Call will perform the specified procedure against the RPC endpoint.
func (c *Client) Call(ctx context.Context, proc Procedure) error {
	// check context
	if ctx == nil {
		ctx = context.Background()
	}

	// get meta
	meta := GetMeta(proc)

	// prepare url
	url := fmt.Sprintf("%s/%s", strings.TrimRight(c.base, "/"), meta.Name)

	// encode procedure
	buf, err := meta.Coding.Marshal(proc)
	if err != nil {
		return err
	}

	// create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return err
	}

	// set content type
	req.Header.Set("Content-Type", mimeTypes[meta.Coding])

	// perform request
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}

	// ensure body is closed
	defer res.Body.Close()

	// read body
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	// check code
	if res.StatusCode != 200 {
		// unmarshal error
		var rpcError Error
		err = meta.Coding.Unmarshal(body, &rpcError)
		if err != nil {
			return ErrorFromStatus(res.StatusCode, "")
		}

		return &rpcError
	}

	// decode response
	err = meta.Coding.Unmarshal(body, proc)
	if err != nil {
		return err
	}

	return nil
}
