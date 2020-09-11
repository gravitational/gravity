/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"

	etcd "github.com/coreos/etcd/client"
	"github.com/gravitational/trace"
)

// FindETCDMemberID finds Member ID by node name in the output from etcd member list:
//
//    6e3bd23ae5f1eae0: name=node2 peerURLs=http://localhost:23802 clientURLs=http://127.0.0.1:23792
//    924e2e83e93f2560: name=node3 peerURLs=http://localhost:23803 clientURLs=http://127.0.0.1:23793
//    a8266ecf031671f3: name=node1 peerURLs=http://localhost:23801 clientURLs=http://127.0.0.1:23791
//
func FindETCDMemberID(output, name string) (string, error) {
	nameChunk := fmt.Sprintf("name=%v", name)
	scanner := bufio.NewScanner(strings.NewReader(output))
	var line string
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if strings.Contains(text, nameChunk) {
			line = text
			break
		}
	}
	if line == "" {
		return "", trace.NotFound("%v not found", name)
	}

	parts := strings.SplitN(line, ": ", 2)
	if len(parts) != 2 {
		return "", trace.BadParameter("%v bad format of '%v'", name, line)
	}

	return parts[0], nil
}

// EtcdInitialCluster interprets the output of etcdctl member list
// as a comma-separated list of name:ip pairs.
func EtcdInitialCluster(memberListOutput string) (string, error) {
	var initialCluster []string

	scanner := bufio.NewScanner(strings.NewReader(memberListOutput))
	for scanner.Scan() {
		match := reMemberList.FindStringSubmatch(scanner.Text())
		if len(match) != 3 {
			continue
		}

		initialCluster = append(initialCluster,
			fmt.Sprintf("%s:%s", match[1], match[2]))
	}

	if err := scanner.Err(); err != nil {
		return "", trace.Wrap(err)
	}

	return strings.Join(initialCluster, ","), nil
}

// EtcdParseMemberList parses "etcdctl member list" output
func EtcdParseMemberList(memberListOutput string) (EtcdMemberList, error) {
	var result EtcdMemberList
	scanner := bufio.NewScanner(strings.NewReader(memberListOutput))
	for scanner.Scan() {
		match := reMemberList.FindStringSubmatch(scanner.Text())
		if len(match) != 3 {
			continue
		}
		result = append(result, EtcdMember{
			Name:     match[1],
			PeerURLs: match[2],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, trace.Wrap(err)
	}
	return result, nil
}

// EtcdMemberList represents parsed "etcdctl member list" output
type EtcdMemberList []EtcdMember

// EtcdMember describes an etcd member from "etcdctl member list" output
type EtcdMember struct {
	// Name is etcd member name
	Name string `json:"name"`
	// PeerURLs is etcd peer URLs
	PeerURLs string `json:"peer_urls"`
}

// HasMember returns true if member list contains specified member
func (l EtcdMemberList) HasMember(name string) bool {
	for _, m := range l {
		if m.Name == name {
			return true
		}
	}
	return false
}

// EtcdHasMember returns the peer given its peerURL.
// Returns nil if no peer exists with the specified peerURL
func EtcdHasMember(peers []etcd.Member, peerURL string) *etcd.Member {
	for _, peer := range peers {
		for _, url := range peer.PeerURLs {
			if url == peerURL {
				return &peer
			}
		}
	}
	return nil
}

var (
	// reMemberList parses name and peer URL from the output of 'etcdctl member list' command
	reMemberList = regexp.MustCompile("name=([^ ]+) peerURLs=https?://([^:]+)")
)
