package provider

import (
	"github.com/hashicorp/terraform/helper/schema"
)

// ExpandStringSet takes a TF schema.Set and converts to a string slice
func ExpandStringSet(configured *schema.Set) []string {
	return ExpandStringList(configured.List())
}

// ExpandStringList takes a TF string list and converts to a string slice
func ExpandStringList(configured []interface{}) []string {
	vs := make([]string, 0, len(configured))
	for _, v := range configured {
		vs = append(vs, v.(string))
	}
	return vs
}

// ExpandStringMap takes a TF map and converts to a map of string[string]
func ExpandStringMap(v map[string]interface{}) map[string]string {
	m := make(map[string]string)
	for k, val := range v {
		m[k] = val.(string)
	}
	return m
}
