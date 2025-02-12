// Copyright 2018 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"encoding/json"
	"fmt"

	"github.com/pingcap/tidb-operator/pkg/apis/pingcap/v1alpha1"
	corelisterv1 "k8s.io/client-go/listers/core/v1"
)

type CaptureStatus struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	IsOwner bool   `json:"is_owner"`
}

// TiCDCControlInterface is the interface that knows how to manage ticdc captures
type TiCDCControlInterface interface {
	// GetStatus returns ticdc's status
	GetStatus(tc *v1alpha1.TidbCluster, ordinal int32) (*CaptureStatus, error)
}

// defaultTiCDCControl is default implementation of TiCDCControlInterface.
type defaultTiCDCControl struct {
	httpClient
	// for unit test only
	testURL string
}

// NewDefaultTiCDCControl returns a defaultTiCDCControl instance
func NewDefaultTiCDCControl(secretLister corelisterv1.SecretLister) *defaultTiCDCControl {
	return &defaultTiCDCControl{httpClient: httpClient{secretLister: secretLister}}
}

func (c *defaultTiCDCControl) GetStatus(tc *v1alpha1.TidbCluster, ordinal int32) (*CaptureStatus, error) {
	httpClient, err := c.getHTTPClient(tc)
	if err != nil {
		return nil, err
	}

	baseURL := c.getBaseURL(tc, ordinal)
	url := fmt.Sprintf("%s/status", baseURL)
	body, err := getBodyOK(httpClient, url)
	if err != nil {
		return nil, err
	}

	status := CaptureStatus{}
	err = json.Unmarshal(body, &status)
	return &status, err
}

func (c *defaultTiCDCControl) getBaseURL(tc *v1alpha1.TidbCluster, ordinal int32) string {
	if c.testURL != "" {
		return c.testURL
	}

	tcName := tc.GetName()
	ns := tc.GetNamespace()
	scheme := tc.Scheme()
	hostName := fmt.Sprintf("%s-%d", TiCDCMemberName(tcName), ordinal)

	return fmt.Sprintf("%s://%s.%s.%s:8301", scheme, hostName, TiCDCPeerMemberName(tcName), ns)
}

// FakeTiCDCControl is a fake implementation of TiCDCControlInterface.
type FakeTiCDCControl struct {
	getStatus func(tc *v1alpha1.TidbCluster, ordinal int32) (*CaptureStatus, error)
}

// NewFakeTiCDCControl returns a FakeTiCDCControl instance
func NewFakeTiCDCControl() *FakeTiCDCControl {
	return &FakeTiCDCControl{}
}

// SetHealth set health info for FakeTiCDCControl
func (c *FakeTiCDCControl) MockGetStatus(mockfunc func(tc *v1alpha1.TidbCluster, ordinal int32) (*CaptureStatus, error)) {
	c.getStatus = mockfunc
}

func (c *FakeTiCDCControl) GetStatus(tc *v1alpha1.TidbCluster, ordinal int32) (*CaptureStatus, error) {
	if c.getStatus == nil {
		return nil, fmt.Errorf("undefined")
	}
	return c.getStatus(tc, ordinal)
}
