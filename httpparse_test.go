package httpparse_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/lag13/httpparse"
)

// errReadCloser is a ReadCloser that can error when read or closed.
type errReadCloser struct {
	data     string
	readErr  error
	closeErr error
}

func (e errReadCloser) Read(b []byte) (int, error) {
	if e.readErr == nil {
		return copy(b, e.data), io.EOF
	}
	return 0, e.readErr
}

func (e errReadCloser) Close() error {
	return e.closeErr
}

// TestGetRawBody tests that getting the raw http response body gets
// the body and returns any expected errors.
func TestGetRawBody(t *testing.T) {
	tests := []struct {
		testScenario   string
		resp           *http.Response
		reqErr         error
		expectStatuses []int
		readLimit      int64
		wantBody       string
		wantErr        string
	}{
		{
			testScenario:   "there was an error when sending the request",
			resp:           nil,
			reqErr:         errors.New("unexpected error with sending request"),
			expectStatuses: nil,
			readLimit:      0,
			wantBody:       "",
			wantErr:        "sending request: unexpected error with sending request",
		},
		{
			testScenario: "there was an error when reading the response body",
			resp: &http.Response{
				Body: errReadCloser{readErr: errors.New("some read err")},
			},
			reqErr:         nil,
			expectStatuses: nil,
			readLimit:      0,
			wantBody:       "",
			wantErr:        "reading response body: some read err",
		},
		{
			testScenario: "the response status code was unexpected",
			resp: &http.Response{
				StatusCode: 999,
				Body:       ioutil.NopCloser(strings.NewReader("woa there")),
			},
			reqErr:         nil,
			expectStatuses: []int{200},
			readLimit:      0,
			wantBody:       "",
			wantErr:        "got status code 999 but wanted 200, body: woa there",
		},
		{
			testScenario: "the response status code was unexpected (when expecting multiple status codes)",
			resp: &http.Response{
				StatusCode: 999,
				Body:       ioutil.NopCloser(strings.NewReader("woa there")),
			},
			reqErr:         nil,
			expectStatuses: []int{200, 888},
			readLimit:      0,
			wantBody:       "",
			wantErr:        "got status code 999 but wanted one of [200 888], body: woa there",
		},
		{
			testScenario: "the response body exceeded the limit so an error is returned",
			resp: &http.Response{
				StatusCode: 400,
				Body:       ioutil.NopCloser(strings.NewReader("a reeaaaallllly loooooooong responnnnnnssssseeeeee bodyyyyyyyy")),
			},
			reqErr:         nil,
			expectStatuses: []int{400},
			readLimit:      19,
			wantBody:       "",
			wantErr:        "ioutil.ReadAll() is used to read the response body and we limit how much it can read because nothing is infinite. The response body contained more than the limit of 19 bytes. Either increase the limit or parse the response body another way",
		},
		{
			testScenario: "the raw response body is returned",
			resp: &http.Response{
				StatusCode: 400,
				Body:       ioutil.NopCloser(strings.NewReader("hello there buddy")),
			},
			reqErr:         nil,
			expectStatuses: []int{400},
			readLimit:      0,
			wantBody:       "hello there buddy",
			wantErr:        "",
		},
	}
	for i, test := range tests {
		errorMsg := func(str string, args ...interface{}) {
			t.Helper()
			t.Errorf("Running test %d, where %s:\n"+str, append([]interface{}{i, test.testScenario}, args...)...)
		}

		var body []byte
		var err error
		if test.readLimit == 0 {
			body, err = httpparse.RawBody(test.resp, test.reqErr, test.expectStatuses)
		} else {
			body, err = httpparse.RawBody(test.resp, test.reqErr, test.expectStatuses, test.readLimit)
		}

		if test.wantErr == "" && err != nil {
			errorMsg("got a non-nil error: %v", err)
		} else if got, want := fmt.Sprintf("%v", err), test.wantErr; want != "" && !strings.Contains(got, want) {
			errorMsg("got error message: %s, wanted message to contain the string: %s", got, want)
		}
		if got, want := string(body), test.wantBody; got != want {
			errorMsg("got body\n  %s\nwanted\n  %s", got, want)
		}
	}
}

type structuredJSON struct {
	ValueOne string `json:"value_one"`
	ValueTwo int    `json:"value_two"`
}

// TestParseJSONResponse tests that parsing a http response with a
// JSON body returns an error when expected and populates the
// unmarshals the raw JSON into the provided value.
func TestParseJSONResponse(t *testing.T) {
	tests := []struct {
		testScenario   string
		resp           *http.Response
		reqErr         error
		expectStatuses []int
		readLimit      int64
		wantData       structuredJSON
		wantErr        string
	}{
		{
			testScenario:   "there was an error when sending the request",
			resp:           nil,
			reqErr:         errors.New("unexpected error with sending request"),
			expectStatuses: nil,
			readLimit:      0,
			wantData:       structuredJSON{},
			wantErr:        "sending request: unexpected error with sending request",
		},
		{
			testScenario: "the response status code was unexpected",
			resp: &http.Response{
				StatusCode: 999,
				Body:       ioutil.NopCloser(strings.NewReader("woa there")),
			},
			reqErr:         nil,
			expectStatuses: []int{200},
			readLimit:      0,
			wantData:       structuredJSON{},
			wantErr:        "got status code 999 but wanted 200",
		},
		{
			testScenario: "the response status code was unexpected (when expecting multiple status codes)",
			resp: &http.Response{
				StatusCode: 999,
				Body:       ioutil.NopCloser(strings.NewReader("woa there")),
			},
			reqErr:         nil,
			expectStatuses: []int{200, 888},
			readLimit:      0,
			wantData:       structuredJSON{},
			wantErr:        "got status code 999 but wanted one of [200 888], body: woa there",
		},
		{
			testScenario: "the response status code was unexpected and there was an error when reading the response body",
			resp: &http.Response{
				StatusCode: 999,
				Body:       errReadCloser{readErr: errors.New("some read err")},
			},
			reqErr:         nil,
			expectStatuses: []int{200},
			readLimit:      0,
			wantData:       structuredJSON{},
			wantErr:        "got status code 999 but wanted 200, also an error occurred when reading the response body: some read err",
		},
		{
			testScenario: "the response status code was unexpected and there was more response body that we didn't read",
			resp: &http.Response{
				StatusCode: 999,
				Body:       ioutil.NopCloser(strings.NewReader(strings.Repeat("z", 1<<20+1))),
			},
			reqErr:         nil,
			expectStatuses: []int{200},
			readLimit:      0,
			wantData:       structuredJSON{},
			wantErr:        "got status code 999 but wanted 200, the first 1048576 bytes of the response body are: zzz",
		},
		{
			testScenario: "we get an error when unmarshalling the data",
			resp: &http.Response{
				StatusCode: 400,
				Body:       ioutil.NopCloser(strings.NewReader(`lats`)),
			},
			reqErr:         nil,
			expectStatuses: []int{400},
			readLimit:      0,
			wantData:       structuredJSON{},
			wantErr:        "unmarshalling response body: invalid character 'l'",
		},
		{
			testScenario: "we get the structured data and no error",
			resp: &http.Response{
				StatusCode: 400,
				Body:       ioutil.NopCloser(strings.NewReader(`{"value_one":"hello there", "value_two":42}`)),
			},
			reqErr:         nil,
			expectStatuses: []int{400},
			readLimit:      0,
			wantData: structuredJSON{
				ValueOne: "hello there",
				ValueTwo: 42,
			},
			wantErr: "",
		},
	}
	for i, test := range tests {
		errorMsg := func(str string, args ...interface{}) {
			t.Helper()
			t.Errorf("Running test %d, where %s:\n"+str, append([]interface{}{i, test.testScenario}, args...)...)
		}

		var data structuredJSON
		err := httpparse.JSON(test.resp, test.reqErr, test.expectStatuses, &data)

		if test.wantErr == "" && err != nil {
			errorMsg("got a non-nil error: %v", err)
		} else if got, want := fmt.Sprintf("%v", err), test.wantErr; want != "" && !strings.Contains(got, want) {
			errorMsg("got error message: %s, wanted message to contain the string: %s", got, want)
		}
		if got, want := data, test.wantData; got != want {
			errorMsg("got data %+v, wanted %+v", got, want)
		}
	}
}
