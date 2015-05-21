package rvt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/docker/machine/log"
)

type RivetAPI struct {
	endpoint string
}

type ApiResponse struct {
	StatusCode int    `json:"status_code,omitempty"`
	Response   string `json:"response,omitempty"`
}

func NewRivetAPI(endpoint string) (*RivetAPI, error) {
	return &RivetAPI{
		endpoint: endpoint,
	}, nil
}

func (r *RivetAPI) getURL(p string) string {
	return r.endpoint + p
}

func (r *RivetAPI) doRequest(method string, p string, params *url.Values, body io.Reader) (*http.Response, error) {
	u := fmt.Sprintf("%s?%s", r.getURL(p), params.Encode())

	log.Debugf("rivet request: method=%s url=%s", method, u)

	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}

	return client.Do(req)
}

func (r *RivetAPI) Create(name string, key []byte, cpu int, memory int, storage int) (*ApiResponse, error) {
	params := &url.Values{}
	params.Add("name", name)
	params.Add("cpu", fmt.Sprintf("%d", cpu))
	params.Add("memory", fmt.Sprintf("%d", memory))
	params.Add("storage", fmt.Sprintf("%d", storage))

	buf := bytes.NewBuffer(key)

	resp, err := r.doRequest("POST", "/create", params, buf)
	if err != nil {
		return nil, err
	}

	var apiResponse ApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, err
	}

	return &apiResponse, nil
}

func (r *RivetAPI) GetState(name string) (*ApiResponse, error) {
	params := &url.Values{}
	params.Add("name", name)

	resp, err := r.doRequest("GET", "/state", params, nil)
	if err != nil {
		return nil, err
	}

	var apiResponse ApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, err
	}

	return &apiResponse, nil
}

func (r *RivetAPI) GetIP(name string) (*ApiResponse, error) {
	params := &url.Values{}
	params.Add("name", name)

	resp, err := r.doRequest("GET", "/ip", params, nil)
	if err != nil {
		return nil, err
	}

	var apiResponse ApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, err
	}

	return &apiResponse, nil
}

func (r *RivetAPI) Remove(name string) (*ApiResponse, error) {
	params := &url.Values{}
	params.Add("name", name)

	resp, err := r.doRequest("GET", "/remove", params, nil)
	if err != nil {
		return nil, err
	}

	var apiResponse ApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, err
	}

	return &apiResponse, nil
}

func (r *RivetAPI) Kill(name string) (*ApiResponse, error) {
	params := &url.Values{}
	params.Add("name", name)

	resp, err := r.doRequest("GET", "/kill", params, nil)
	if err != nil {
		return nil, err
	}

	var apiResponse ApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, err
	}

	return &apiResponse, nil
}

func (r *RivetAPI) Restart(name string) (*ApiResponse, error) {
	params := &url.Values{}
	params.Add("name", name)

	resp, err := r.doRequest("GET", "/restart", params, nil)
	if err != nil {
		return nil, err
	}

	var apiResponse ApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, err
	}

	return &apiResponse, nil
}

func (r *RivetAPI) Start(name string) (*ApiResponse, error) {
	params := &url.Values{}
	params.Add("name", name)

	resp, err := r.doRequest("GET", "/start", params, nil)
	if err != nil {
		return nil, err
	}

	var apiResponse ApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, err
	}

	return &apiResponse, nil
}

func (r *RivetAPI) Stop(name string) (*ApiResponse, error) {
	params := &url.Values{}
	params.Add("name", name)

	resp, err := r.doRequest("GET", "/stop", params, nil)
	if err != nil {
		return nil, err
	}

	var apiResponse ApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, err
	}

	return &apiResponse, nil
}
