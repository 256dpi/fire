package fire

import (
	"crypto/tls"
	"io/ioutil"
	"net"
	"net/http"
)

func freeAddr() string {
	listener, err := net.Listen("tcp", ":")
	if err != nil {
		panic(err)
	}

	listener.Close()

	return listener.Addr().String()
}

func testRequest(url string) (string, *http.Response, error) {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}}

	res, err := client.Get(url)
	if err != nil {
		return "", nil, err
	}

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", nil, err
	}

	return string(buf), res, nil
}
