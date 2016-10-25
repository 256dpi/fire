package fire

import (
	"io/ioutil"
	"net/http"
)

func testRequest(url string) (string, *http.Response, error) {
	res, err := http.Get(url)
	if err != nil {
		return "", nil, err
	}

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", nil, err
	}

	return string(buf), res, nil
}
