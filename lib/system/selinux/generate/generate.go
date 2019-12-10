package main

import (
	"github.com/gravitational/gravity/lib/system/selinux"

	"github.com/shurcooL/vfsgen"
	log "github.com/sirupsen/logrus"
)

func main() {
	err := vfsgen.Generate(selinux.Policy, vfsgen.Options{
		Filename:     "policy_embed.go",
		BuildTags:    "selinux_embed",
		PackageName:  "selinux",
		VariableName: "Policy",
	})
	if err != nil {
		log.Fatalln(err)
	}
}
