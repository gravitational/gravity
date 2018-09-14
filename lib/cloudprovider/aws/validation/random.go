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

package validation

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go/aws"
)

// abc is the random text generation alphabet
// As the validation process requires to reference named AWS resources, it needs
// to be able to generate names that comply with the scheme expected by AWS API.
// The alphabet defines the character set used by the AWS API to generate resource names.
const abc = "0123456789abcdef"

// dryRun turns on dry run operation for most of the AWS API calls
var dryRun = aws.Bool(true)

// dummyValue generates a random name of length dummyNameLen
func dummyValue(prefix string) *string {
	const dummyNameLen = 5
	return dummyValueWithLen(prefix, dummyNameLen)
}

// dummyValueWithLen generates a random name of arbitrary length
func dummyValueWithLen(prefix string, n int) *string {
	randSrc := rand.NewSource(time.Now().UnixNano())
	randomSuffix := randomString(n, randSrc)
	return aws.String(fmt.Sprintf("%v%v", prefix, randomSuffix))
}

// randomBytes generates a byte chunk of randomness using abc as alphabet
func randomBytes(n int, rand rand.Source) []byte {
	r := make([]byte, n)
	for i := 0; i < n; i++ {
		r[i] = abc[int(rand.Int63())%len(abc)]
	}
	return r
}

// randomString generates a string of randomness using abc as alphabet
func randomString(n int, rand rand.Source) string {
	b := randomBytes(n, rand)
	return string(b)
}
