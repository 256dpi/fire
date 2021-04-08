package nitro

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/256dpi/xo"
)

// TODO: Automatically retry until context is cancelled?

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

	// trace request
	ctx, span := xo.Trace(ctx, "CALL "+meta.Name)
	defer span.End()

	// pre validate
	err := proc.Validate()
	if err != nil {
		return xo.W(err)
	}

	// prepare url
	url := fmt.Sprintf("%s/%s", strings.TrimRight(c.base, "/"), meta.Name)

	// encode procedure
	buf, err := meta.Coding.Marshal(proc)
	if err != nil {
		return err
	}

	// TODO: Set trace headers.

	// create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return xo.W(err)
	}

	// set content type
	req.Header.Set("Content-Type", meta.Coding.MimeType())

	// perform request
	res, err := c.http.Do(req)
	if err != nil {
		return xo.W(err)
	}

	// ensure body is closed
	defer res.Body.Close()

	// read body
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return xo.W(err)
	}

	// check code
	if res.StatusCode != 200 {
		// unmarshal error
		var rpcError Error
		err = meta.Coding.Unmarshal(body, &rpcError)
		if err != nil {
			return xo.W(ErrorFromStatus(res.StatusCode, ""))
		}

		return xo.W(&rpcError)
	}

	// decode response
	err = meta.Coding.Unmarshal(body, proc)
	if err != nil {
		return err
	}

	// post validate
	err = proc.Validate()
	if err != nil {
		return xo.W(err)
	}

	return nil
}
