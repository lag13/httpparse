// Package httpparse contains utilities for parsing http responses.
package httpparse

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

func contains(xs []int, y int) bool {
	for _, x := range xs {
		if x == y {
			return true
		}
	}
	return false
}

// RawBody takes a response, the error returned from sending the http
// request that resulted in the response, and a list of expected
// status codes. It returns the response body as a []byte and an error
// if something goes wrong (like status codes were not what was
// expected). It was created to consolidate the logic of checking that
// the response code is what we expect, closing the response, etc...
func RawBody(resp *http.Response, requestErr error, wantStatuses []int, readLimit ...int64) (body []byte, err error) {
	if requestErr != nil {
		return nil, fmt.Errorf("sending request: %v", requestErr)
	}
	// From what I've gathered, checking an error returned from
	// closing a resource that you only read from (like an HTTP
	// response body) never yields an actionable error:
	// https://github.com/kisielk/errcheck/issues/55#issuecomment-68296619,
	// https://groups.google.com/d/msg/Golang-Nuts/7Ek7Uo7vSqU/0UfsWpIFQ_YJ,
	// https://www.reddit.com/r/golang/comments/3735so/do_we_have_to_check_for_errors_when_we_call_close/.
	// The documentation on net/http also does not check this
	// error: https://golang.org/pkg/net/http/. Since this is the
	// case it feels like the response body should not satisfy the
	// io.ReadCloser interface, oh well.
	defer resp.Body.Close()
	// People say that using ioutil.ReadAll() is not ideal
	// (https://www.reddit.com/r/golang/comments/2cdu7s/how_do_i_avoid_using_ioutilreadall/,
	// http://jmoiron.net/blog/crossing-streams-a-love-letter-to-ioreader/)
	// but I'm still not sure under exactly what conditions it is
	// bad. I get that there is no infinity when it comes to
	// memory so it could use too much memory and crash the
	// process (or something like that). But this function was
	// created with the goal of being a helper when parsing
	// responses from APIs that return JSON where we need access
	// to the raw data (like if you wanted to cache it somewhere)
	// and in my experience those APIs never return large amounts
	// of data. I also feel it is useful to have the "raw" data in
	// case something goes wrong with the JSON parsing because
	// then you can see the data that the parsing failed on. Just
	// in case though I am limiting the amount of data that can be
	// read. The default limit (30 MB) is arbitrary and can be
	// changed if desired.
	maxBytes := int64(1 << 20 * 30)
	if len(readLimit) > 0 {
		maxBytes = readLimit[0]
	}
	limitedReader := &io.LimitedReader{
		R: resp.Body,
		N: maxBytes + 1,
	}
	body, err = ioutil.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %v", err)
	}
	if limitedReader.N <= 0 {
		return nil, fmt.Errorf("ioutil.ReadAll() is used to read the response body and we limit how much it can read because nothing is infinite. The response body contained more than the limit of %d bytes. Either increase the limit or parse the response body another way", maxBytes)
	}
	if got, wants := resp.StatusCode, wantStatuses; !contains(wants, got) {
		errStr := fmt.Sprintf("got status code %d but wanted one of %v", got, wants)
		if len(wants) == 1 {
			errStr = fmt.Sprintf("got status code %d but wanted %d", got, wants[0])
		}
		return nil, fmt.Errorf("%s, body: %s", errStr, body)
	}
	return body, nil
}

// JSON checks parses a http response who's body contains JSON. Most
// of this logic revolves around trying to produce nice helpful error
// messages. For example, if the response status code is unexpected
// then you can't possibly know how to unmarshal that response body so
// we try to read part of the response body and return that as part of
// the error so there is context. But reading the response can fail,
// or we might not read ALL of the response, etc... and we just want a
// nice clear error message for all those cases.
func JSON(resp *http.Response, requestErr error, wantStatuses []int, v interface{}) error {
	if requestErr != nil {
		return fmt.Errorf("sending request: %v", requestErr)
	}
	defer resp.Body.Close()
	if got, wants := resp.StatusCode, wantStatuses; !contains(wants, got) {
		maxBytes := int64(1 << 20)
		limitedReader := &io.LimitedReader{
			R: resp.Body,
			N: maxBytes + 1,
		}
		body, readErr := ioutil.ReadAll(limitedReader)
		err := fmt.Errorf("got status code %d but wanted one of %v", got, wants)
		if len(wants) == 1 {
			err = fmt.Errorf("got status code %d but wanted %d", got, wants[0])
		}
		if readErr != nil {
			return fmt.Errorf("%v, also an error occurred when reading the response body: %v", err, readErr)
		}
		if limitedReader.N <= 0 {
			return fmt.Errorf("%v, the first %d bytes of the response body are: %s", err, maxBytes, body)
		}
		return fmt.Errorf("%v, body: %s", err, body)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("unmarshalling response body: %v", err)
	}
	return nil
}