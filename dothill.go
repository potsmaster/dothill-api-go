package dothill

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"k8s.io/klog"
)

// Client : Can be used to request the dothill API
type Client struct {
	Username   string
	Password   string
	Addr       string
	HTTPClient http.Client
	sessionKey string
}

// NewClient : Creates a dothill client by setting up its HTTP client
func NewClient() *Client {
	return &Client{
		HTTPClient: http.Client{
			Timeout: time.Duration(15 * time.Second),
			Transport: &http.Transport{
				// Proxy: http.ProxyURL(proxy),
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	}
}

// Request : Execute the given request with client's configuration
func (client *Client) Request(endpoint string) (*Response, *ResponseStatus, error) {
	return client.request(&Request{Endpoint: endpoint})
}

func (client *Client) request(req *Request) (*Response, *ResponseStatus, error) {
	isLoginReq := strings.Contains(req.Endpoint, "login")
	if !isLoginReq {
		if len(client.sessionKey) == 0 {
			klog.V(1).Info("no session key stored, authenticating before sending request")
			err := client.Login()
			if err != nil {
				return nil, NewErrorStatus("login failed"), err
			}
		}

		klog.Infof("-> GET %s", req.Endpoint)
	} else {
		klog.Infof("-> GET /login/<hidden>")
	}

	raw, code, err := req.execute(client)
	if code == 401 && !isLoginReq {
		klog.V(1).Info("session key may have expired, trying to re-login")
		err = client.Login()
		if err != nil {
			return nil, NewErrorStatus("re-login failed"), err
		}
		klog.V(1).Info("re-login succeed, re-trying request")
		raw, _, err = req.execute(client)
	}
	if err != nil {
		return nil, NewErrorStatus("request failed"), err
	}

	res, err := NewResponse(raw)
	if err != nil {
		if res != nil {
			return res, res.GetStatus(), err
		}

		return nil, NewErrorStatus("corrupted response"), err
	}

	status := res.GetStatus()
	if !isLoginReq {
		klog.Infof("<- [%d %s] %s", status.ReturnCode, status.ResponseType, status.Response)
	} else {
		klog.Infof("<- [%d %s] <hidden>", status.ReturnCode, status.ResponseType)
	}
	if status.ResponseTypeNumeric != 0 {
		return res, status, fmt.Errorf("Dothill API returned non-zero code %d (%s)", status.ReturnCode, status.Response)
	}

	return res, status, nil
}
