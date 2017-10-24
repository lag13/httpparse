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
		name           string
		resp           *http.Response
		expectStatuses []int
		readLimit      int64
		wantBody       string
		wantErr        string
	}{
		{
			name: "error reading response body",
			resp: &http.Response{
				Body: errReadCloser{readErr: errors.New("some read err")},
			},
			expectStatuses: nil,
			readLimit:      0,
			wantBody:       "",
			wantErr:        "reading response body: some read err",
		},
		{
			name: "unexpected response status code",
			resp: &http.Response{
				StatusCode: 999,
				Body:       ioutil.NopCloser(strings.NewReader("woa there")),
			},
			expectStatuses: []int{200},
			readLimit:      0,
			wantBody:       "",
			wantErr:        "got status code 999 but wanted 200, body: woa there",
		},
		{
			name: "unexpected status code when expecting multiple status codes",
			resp: &http.Response{
				StatusCode: 999,
				Body:       ioutil.NopCloser(strings.NewReader("woa there")),
			},
			expectStatuses: []int{200, 888},
			readLimit:      0,
			wantBody:       "",
			wantErr:        "got status code 999 but wanted one of [200 888], body: woa there",
		},
		{
			name: "response body exceeded the limit",
			resp: &http.Response{
				StatusCode: 400,
				Body:       ioutil.NopCloser(strings.NewReader("a reeaaaallllly loooooooong responnnnnnssssseeeeee bodyyyyyyyy")),
			},
			expectStatuses: []int{400},
			readLimit:      19,
			wantBody:       "",
			wantErr:        "ioutil.ReadAll() is used to read the response body and we limit how much it can read because nothing is infinite. The response body contained more than the limit of 19 bytes. Either increase the limit or parse the response body another way",
		},
		{
			name: "returned raw response body",
			resp: &http.Response{
				StatusCode: 400,
				Body:       ioutil.NopCloser(strings.NewReader("hello there buddy")),
			},
			expectStatuses: []int{400},
			readLimit:      0,
			wantBody:       "hello there buddy",
			wantErr:        "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var body []byte
			var err error
			if test.readLimit == 0 {
				body, err = httpparse.RawBody(test.resp, test.expectStatuses)
			} else {
				body, err = httpparse.RawBody(test.resp, test.expectStatuses, test.readLimit)
			}

			if test.wantErr == "" && err != nil {
				t.Errorf("got a non-nil error: %v", err)
			} else if got, want := fmt.Sprintf("%v", err), test.wantErr; want != "" && !strings.Contains(got, want) {
				t.Errorf("got error message: %s, wanted message to contain the string: %s", got, want)
			}
			if got, want := string(body), test.wantBody; got != want {
				t.Errorf("got body\n  %s\nwanted\n  %s", got, want)
			}
		})
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
		name         string
		resp         *http.Response
		expectStatus int
		readLimit    int64
		wantData     structuredJSON
		wantErr      string
	}{
		{
			name: "unexpected response status code",
			resp: &http.Response{
				StatusCode: 999,
				Body:       ioutil.NopCloser(strings.NewReader("woa there")),
			},
			expectStatus: 200,
			readLimit:    0,
			wantData:     structuredJSON{},
			wantErr:      "got status code 999 but wanted 200, body: woa there",
		},
		{
			name: "unexpected response status code and error reading response body",
			resp: &http.Response{
				StatusCode: 999,
				Body:       errReadCloser{readErr: errors.New("some read err")},
			},
			expectStatus: 200,
			readLimit:    0,
			wantData:     structuredJSON{},
			wantErr:      "got status code 999 but wanted 200, also an error occurred when reading the response body: some read err",
		},
		{
			name: "unexpected response status code did not read all of response body",
			resp: &http.Response{
				StatusCode: 999,
				Body:       ioutil.NopCloser(strings.NewReader(strings.Repeat("z", 1<<20+1))),
			},
			expectStatus: 200,
			readLimit:    0,
			wantData:     structuredJSON{},
			wantErr:      "got status code 999 but wanted 200, the first 1048576 bytes of the response body are: zzz",
		},
		{
			name: "error when unmarshalling response body",
			resp: &http.Response{
				StatusCode: 400,
				Body:       ioutil.NopCloser(strings.NewReader(`lats`)),
			},
			expectStatus: 400,
			readLimit:    0,
			wantData:     structuredJSON{},
			wantErr:      "unmarshalling response body: invalid character 'l'",
		},
		{
			name: "got the structured data",
			resp: &http.Response{
				StatusCode: 400,
				Body:       ioutil.NopCloser(strings.NewReader(`{"value_one":"hello there", "value_two":42}`)),
			},
			expectStatus: 400,
			readLimit:    0,
			wantData: structuredJSON{
				ValueOne: "hello there",
				ValueTwo: 42,
			},
			wantErr: "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var data structuredJSON
			err := httpparse.JSON(test.resp, test.expectStatus, &data)

			if test.wantErr == "" && err != nil {
				t.Errorf("got a non-nil error: %v", err)
			} else if got, want := fmt.Sprintf("%v", err), test.wantErr; want != "" && !strings.Contains(got, want) {
				t.Errorf("got error message: %s, wanted message to contain the string: %s", got, want)
			}
			if got, want := data, test.wantData; got != want {
				t.Errorf("got data %+v, wanted %+v", got, want)
			}
		})
	}
}
