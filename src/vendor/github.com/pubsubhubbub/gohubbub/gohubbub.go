// Copyright 2014 Daniel Pupius

// Package gohubbub provides a PubSubHubbub subscriber client.  It will request
// subscriptions from a hub and field responses as required by the prootcol.
// Update notifications will be forwarded to the handler function that is
// registered on subscription.
package gohubbub

// TODO: Renew lease.

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Struct for storing information about a subscription.
type Subscription struct {
	topic    string
	id       int
	handler  func(string, []byte) // Content-Type, ResponseBody
	lease    time.Duration
	verified bool
}

func (s Subscription) String() string {
	return fmt.Sprintf("%s (#%d %s)", s.topic, s.id, s.lease)
}

var NIL_SUBSCRIPTION = &Subscription{}

// A HttpRequester is used to make HTTP requests.  http.Client{} satisfies this
// interface.
type HttpRequester interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

// Client allows you to make PubSubHubbub subscriptions and register callback
// handlers that will be executed when an update is received.
type Client struct {
	// URL of the PubSubHubbub Hub to make requests to.
	hubURL string

	// Hostname or IP address that the client will be served from, should be
	// accessible by the hub. e.g. "push.myhost.com"
	self string

	port          int                      // Which port the server will be started on.
	from          string                   // String passed in the "From" header.
	running       bool                     // Whether the server is running.
	subscriptions map[string]*Subscription // Map of subscriptions.
	httpRequester HttpRequester            // e.g. http.Client{}.
}

func NewClient(hubURL string, self string, port int, from string) *Client {
	return &Client{
		hubURL,
		self,
		port,
		fmt.Sprintf("%s (gohubbub)", from),
		false,
		make(map[string]*Subscription),
		&http.Client{}, // TODO: Use client with Timeout transport.
	}
}

// Subscribe adds a subscription to the client, the handler will be called when
// an update notification is received.  If a handler already exists it will be
// overridden.
func (client *Client) Subscribe(topic string, handler func(string, []byte)) {
	subscription := &Subscription{topic, len(client.subscriptions), handler, 0, false}
	client.subscriptions[topic] = subscription
	if client.running {
		client.makeSubscriptionRequest(subscription)
	}
}

// Unsubscribe sends an unsubscribe notification and removes the subscription.
func (client *Client) Unsubscribe(topic string) {
	if subscription, exists := client.subscriptions[topic]; exists {
		delete(client.subscriptions, topic)
		if client.running {
			client.makeUnsubscribeRequeast(subscription)
		}
	} else {
		log.Printf("Cannot unsubscribe, %s doesn't exist.", topic)
	}
}

func (client *Client) StartServer() {
	client.running = true

	// Trigger subscription requests async.
	for _, subscription := range client.subscriptions {
		go client.makeSubscriptionRequest(subscription)
	}

	// Start the server.
	http.HandleFunc("/", client.handleRequest)
	http.HandleFunc("/callback/", client.handleCallback)
	log.Printf("Starting HTTP server on port %d", client.port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", client.port), nil))
}

// String provides a textual representation of the client's current state.
func (client Client) String() string {
	urls := make([]string, len(client.subscriptions))
	i := 0
	for k, _ := range client.subscriptions {
		urls[i] = k
		i++
	}
	return fmt.Sprintf("%d subscription(s): %v", len(client.subscriptions), urls)
}

func (client *Client) makeSubscriptionRequest(subscription *Subscription) {
	log.Println("Subscribing to", subscription.topic)

	body := url.Values{}
	body.Set("hub.callback", client.formatCallbackURL(subscription.id))
	body.Add("hub.topic", subscription.topic)
	body.Add("hub.mode", "subscribe")
	// body.Add("hub.lease_seconds", "60")

	req, _ := http.NewRequest("POST", client.hubURL, bytes.NewBufferString(body.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("From", client.from)

	resp, err := client.httpRequester.Do(req)

	if err != nil {
		log.Printf("Subscription failed, %s, %s", *subscription, err)

	} else if resp.StatusCode != 202 {
		log.Printf("Subscription failed, %s, status = %s", *subscription, resp.Status)
	}
}

func (client *Client) makeUnsubscribeRequeast(subscription *Subscription) {
	log.Println("Unsubscribing from", subscription.topic)

	body := url.Values{}
	body.Set("hub.callback", client.formatCallbackURL(subscription.id))
	body.Add("hub.topic", subscription.topic)
	body.Add("hub.mode", "unsubscribe")

	req, _ := http.NewRequest("POST", client.hubURL, bytes.NewBufferString(body.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("From", client.from)

	resp, err := client.httpRequester.Do(req)

	if err != nil {
		log.Printf("Unsubscribe failed, %s, %s", *subscription, err)

	} else if resp.StatusCode != 202 {
		log.Printf("Unsubscribe failed, %s status = %d", *subscription, resp.Status)
	}
}

func (client *Client) formatCallbackURL(callback int) string {
	return fmt.Sprintf("http://%s:%d/callback/%d", client.self, client.port, callback)
}

func (client *Client) handleRequest(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
	resp.Write([]byte("Hello!"))
	log.Println("Request Received!")
}

func (client *Client) handleCallback(resp http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	requestBody, err := ioutil.ReadAll(req.Body)

	if err != nil {
		log.Printf("Error reading callback request, %s", err)
		return
	}

	params := req.URL.Query()
	topic := params.Get("hub.topic")

	switch params.Get("hub.mode") {
	case "subscribe":
		if subscription, exists := client.subscriptions[topic]; exists {
			subscription.verified = true
			lease, err := strconv.Atoi(params.Get("hub.lease_seconds"))
			if err == nil {
				subscription.lease = time.Second * time.Duration(lease)
			}

			log.Printf("Subscription verified for %s, lease is %s", topic, subscription.lease)
			resp.Write([]byte(params.Get("hub.challenge")))

		} else {
			log.Printf("Unexpected subscription for %s", topic)
			http.Error(resp, "Unexpected subscription", http.StatusBadRequest)
		}

	case "unsubscribe":
		// We optimistically removed the subscription, so only confirm the
		// unsubscribe if no subscription exists for the topic.
		if _, exists := client.subscriptions[topic]; !exists {
			log.Printf("Unsubscribe confirmed for %s", topic)
			resp.Write([]byte(params.Get("hub.challenge")))

		} else {
			log.Printf("Unexpected unsubscribe for %s", topic)
			http.Error(resp, "Unexpected unsubscribe", http.StatusBadRequest)
		}

	case "denied":
		log.Printf("Subscription denied for %s, reason was %s", topic, params.Get("hub.reason"))
		resp.Write([]byte{})
		// TODO: Don't do anything for now, should probably mark the subscription.

	default:
		subscription, exists := client.subscriptionForPath(req.URL.Path)
		if !exists {
			log.Printf("Callback for unknown subscription: %s", req.URL.String())
			http.Error(resp, "Unknown subscription", http.StatusBadRequest)

		} else {
			log.Printf("Update for %s", subscription)
			resp.Write([]byte{})

			// Asynchronously notify the subscription handler, shouldn't affect response.
			go subscription.handler(req.Header.Get("Content-Type"), requestBody)
		}
	}

}

func (client *Client) subscriptionForPath(path string) (*Subscription, bool) {
	parts := strings.Split(path, "/")
	if len(parts) != 3 {
		return NIL_SUBSCRIPTION, false
	}
	id, err := strconv.Atoi(parts[2])
	if err != nil {
		return NIL_SUBSCRIPTION, false
	}
	for _, subscription := range client.subscriptions {
		if subscription.id == id {
			return subscription, true
		}
	}
	return NIL_SUBSCRIPTION, false
}

// Protocol cheat sheet:
// ---------------------
//
// SUBSCRIBE
// POST to hub
//
// ContentType: application/x-www-form-urlencoded
// From: gohubbub test app
//
// hub.callback The subscriber's callback URL where notifications should be delivered.
// hub.mode "subscribe" or "unsubscribe"
// hub.topic The topic URL that the subscriber wishes to subscribe to or unsubscribe from.
// hub.lease_seconds Number of seconds for which the subscriber would like to have the subscription active. Hubs MAY choose to respect this value or not, depending on their own policies. This parameter MAY be present for unsubscription requests and MUST be ignored by the hub in that case.
//
// Response should be 202 "Accepted"

// CALLBACK - Denial notification
// Request will have the following query params:
// hub.mode=denied
// hub.topic=[URL that was denied]
// hub.reason=[why it was denied (optional)]

// CALLBACK - Verification
// Request will have the following query params:
// hub.mode=subscribe or unsubscribe
// hub.topic=[URL that was denied]
// hub.challenge=[random string]
// hub.lease_seconds=[how long lease will be held]
//
// Response should be 2xx with hub.challenge in response body.
// 400 to reject

// CALLBACK - Update notification
// Content-Type
// Payload may be a diff
// Link header with rel=hub
// Link header rel=self for topic
//
// Response empty 2xx
