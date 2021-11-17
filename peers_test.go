package main

import (
	"testing"

	"github.com/mailgun/gubernator/v2"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestKubernetesPeers_set(t *testing.T) {
	testCases := []struct {
		name     string
		infos    []gubernator.PeerInfo
		expected []string
	}{
		{
			name: "grpc",
			infos: []gubernator.PeerInfo{
				{DataCenter: "", HTTPAddress: "", GRPCAddress: "10.0.0.1:3000", IsOwner: false},
				{DataCenter: "", HTTPAddress: "", GRPCAddress: "10.0.0.2:3000", IsOwner: true},
			},
			expected: []string{"http://10.0.0.1:3000", "http://10.0.0.2:3000"},
		},
		{
			name: "http",
			infos: []gubernator.PeerInfo{
				{DataCenter: "", HTTPAddress: "http://10.0.0.1:3000", GRPCAddress: "", IsOwner: false},
				{DataCenter: "", HTTPAddress: "http://10.0.0.2:3000", GRPCAddress: "", IsOwner: true},
			},
			expected: []string{"http://10.0.0.1:3000", "http://10.0.0.2:3000"},
		},
		{
			name: "an invalid peer",
			infos: []gubernator.PeerInfo{
				{DataCenter: "", HTTPAddress: "", GRPCAddress: "", IsOwner: false},
				{DataCenter: "", HTTPAddress: "http://10.0.0.2:3000", GRPCAddress: "", IsOwner: true},
			},
			expected: []string{"http://10.0.0.2:3000"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var actual []string
			kp := KubernetesPeers{
				setter: func(s ...string) {
					actual = s
				},
				logger: logrus.New(),
			}

			kp.set(tc.infos)

			assert.Equal(t, tc.expected, actual)
		})
	}
}
