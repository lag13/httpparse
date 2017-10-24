/*

Package httpparse provides utilities for parsing http responses.

Motivation

Parsing a http response into some sort of data structure (or even just
pulling out the raw body) follows a very repetitive pattern of:

	1. defer'ring closing the response body.
	2. checking that we got the expected status code.
	3. reading and parsing the response body.
	4. generating error messages if any of these things fail.

It's not much logic but I've repeated it enough to want an abstraction
and even if this package is not used it serves as a nice reference.

*/
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

// RawBody returns the raw http body as a []byte and errors if
// anything goes wrong. It also closes the response body.
func RawBody(resp *http.Response, wantStatuses []int, readLimit ...int64) (body []byte, err error) {
	// From what I've gathered, checking an error returned from
	// closing a resource that you only read from (like a HTTP
	// response body) never yields an actionable error:
	// https://github.com/kisielk/errcheck/issues/55#issuecomment-68296619,
	// https://groups.google.com/d/msg/Golang-Nuts/7Ek7Uo7vSqU/0UfsWpIFQ_YJ,
	// https://www.reddit.com/r/golang/comments/3735so/do_we_have_to_check_for_errors_when_we_call_close/.
	// The documentation on net/http also does not check this
	// error: https://golang.org/pkg/net/http/. If checking the
	// error is not useful it kind of feels like it shouldn't even
	// return an error, oh well.
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
	// of data. Just in case though I am limiting the amount of
	// data that can be read. The default limit (30 MB) is
	// arbitrary and can be changed if desired.
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

// JSON parses a http response who's body contains JSON and closes the
// response body. Most of the logic revolves around trying to produce
// clear error messages when edge cases are hit.
func JSON(resp *http.Response, wantStatus int, v interface{}) error {
	defer resp.Body.Close()
	if got, want := resp.StatusCode, wantStatus; got != want {
		maxBytes := int64(1 << 20)
		limitedReader := &io.LimitedReader{
			R: resp.Body,
			N: maxBytes + 1,
		}
		err := fmt.Errorf("got status code %d but wanted %d", got, want)
		body, readErr := ioutil.ReadAll(limitedReader)
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
