// +build generate_policy

package main

import (
	"log"

	"github.com/gravitational/gravity/lib/system/selinux/internal/policy"

	"github.com/gravitational/vfsgen"
)

func main() {
	err := vfsgen.Generate(policy.Policy, vfsgen.Options{
		Filename:     "policy_embed.go",
		BuildTags:    "selinux_embed",
		PackageName:  "policy",
		VariableName: "Policy",
	})
	if err != nil {
		log.Fatalln(err)
	}
}
