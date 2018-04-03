// Package core provides utility methods that help convert proxy events
// into an http.Request and http.ResponseWriter
package core

import (
	"encoding/base64"
	"errors"
	"net/http"
	"unicode/utf8"

	"github.com/aws/aws-lambda-go/events"
)

const defaultStatusCode = -1

// ProxyResponseWriter implements http.ResponseWriter and adds the method
// necessary to return an events.APIGatewayProxyResponse object
type ProxyResponseWriter struct {
	headers http.Header
	body    []byte
	status  int
}

// NewProxyResponseWriter returns a new ProxyResponseWriter object.
// The object is initialized with an empty map of headers and a
// status code of -1
func NewProxyResponseWriter() *ProxyResponseWriter {
	return &ProxyResponseWriter{
		headers: make(http.Header),
		status:  defaultStatusCode,
	}

}

// Header implementation from the http.ResponseWriter interface.
func (r *ProxyResponseWriter) Header() http.Header {
	return r.headers
}

// Write sets the response body in the object. If no status code
// was set before with the WriteHeader method it sets the status
// for the response to 200 OK.
func (r *ProxyResponseWriter) Write(body []byte) (int, error) {
	r.body = body
	if r.status == -1 {
		r.status = http.StatusOK
	}

	return len(body), nil
}

// WriteHeader sets a status code for the response. This method is used
// for error responses.
func (r *ProxyResponseWriter) WriteHeader(status int) {
	r.status = status
}

// GetProxyResponse converts the data passed to the response writer into
// an events.APIGatewayProxyResponse object.
// Returns a populated proxy response object. If the reponse is invalid, for example
// has no headers or an invalid status code returns an error.
func (r *ProxyResponseWriter) GetProxyResponse() (events.APIGatewayProxyResponse, error) {
	if r.status == defaultStatusCode {
		return events.APIGatewayProxyResponse{}, errors.New("Status code not set on response")
	}

	var output string
	isBase64 := false

	if utf8.Valid(r.body) {
		output = string(r.body)
	} else {
		output = base64.StdEncoding.EncodeToString(r.body)
		isBase64 = true
	}

	proxyHeaders := make(map[string]string)

	for h := range r.headers {
		proxyHeaders[h] = r.headers.Get(h)
	}

	return events.APIGatewayProxyResponse{
		StatusCode:      r.status,
		Headers:         proxyHeaders,
		Body:            output,
		IsBase64Encoded: isBase64,
	}, nil
}
