package httpparse_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/lag13/httpparse"
)

func ExampleRawBody() {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(strings.NewReader(`{"field1":"hello there", "field2":42}`)),
	}
	body, err := httpparse.RawBody(resp, []int{http.StatusOK})
	if err != nil {
		fmt.Println("got error:", err)
	}
	fmt.Printf("got body: %s\n", body)

	// Output: got body: {"field1":"hello there", "field2":42}
}

func ExampleJSON() {
	var structuredBody struct {
		Field1 string `json:"field1"`
		Field2 int    `json:"field2"`
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(strings.NewReader(`{"field1":"hello there", "field2":42}`)),
	}
	if err := httpparse.JSON(resp, http.StatusOK, &structuredBody); err != nil {
		fmt.Println("got error:", err)
	}
	fmt.Println("field1 is:", structuredBody.Field1)
	fmt.Println("field2 is:", structuredBody.Field2)

	// Output: field1 is: hello there
	// field2 is: 42
}
