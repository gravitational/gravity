/*
Copyright 2015 Gravitational, Inc.

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
package schema

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"io"
	"strconv"
	"strings"

	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"
	"gopkg.in/alecthomas/kingpin.v2"
)

func ParseJSON(r io.Reader) (*Config, error) {
	var c *configV1

	if err := json.NewDecoder(r).Decode(&c); err != nil {
		return nil, trace.Wrap(err)
	}
	return newParser().parse(*c)
}

func ParseVariablesJSON(r io.Reader) (*Config, error) {
	var variables []paramSpec

	if err := json.NewDecoder(r).Decode(&variables); err != nil {
		return nil, trace.Wrap(err)
	}
	return newParser().parse(configV1{Params: variables})
}

type Config struct {
	Params []Param
}

func (c *Config) Vars() map[string]string {
	vars := make(map[string]string, len(c.Params))
	for _, p := range c.Params {
		k, v := p.Vars()
		vars[k] = v
	}
	return vars
}

// EnvVars returns a map with environment variables for all parameters
func (c *Config) EnvVars() map[string]string {
	vars := make(map[string]string, len(c.Params))
	for _, p := range c.Params {
		k, v := p.EnvVars()
		vars[k] = v
	}
	return vars
}

// Args returns the list of arguments for all parameters
func (c *Config) Args() []string {
	args := []string{}
	for _, p := range c.Params {
		args = append(args, p.Args()...)
	}
	return args
}

func (c *Config) ParseArgs(args []string) error {
	app := cliApp(c, false)
	_, err := app.Parse(args)
	return err
}

func (c *Config) ParseEnv() error {
	app := cliApp(c, true)
	_, err := app.Parse([]string{})
	return err
}

func (c *Config) ParseVars(vars map[string]string) error {
	for _, p := range c.Params {
		val, ok := vars[p.Name()]
		if !ok {
			if p.Required() {
				return trace.Errorf("missing value for required variable: %v", p.Name())
			} else {
				val = p.Default()
			}
		}
		if err := p.Set(val); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func cliApp(c *Config, useEnv bool) *kingpin.Application {
	app := kingpin.New("app", "Orbit package configuration tool")

	for _, p := range c.Params {
		cliFlag(app, p, useEnv)
	}
	return app
}

func cliFlag(app *kingpin.Application, p Param, useEnv bool) {
	name := p.CLIName()
	f := app.Flag(name, p.Description())
	if p.Required() {
		f = f.Required()
	}
	if p.Default() != "" {
		f = f.Default(p.Default())
	}
	if useEnv {
		f.OverrideDefaultFromEnvar(p.EnvName())
	}
	SetParam(p, f)
}

func SetParam(p Param, s kingpin.Settings) {
	s.SetValue(p)
}

// Param is the command line parameter
type Param interface {
	// Name is the parameter's name
	Name() string
	// CLIName specifies the name of the parameter as given on command line
	CLIName() string
	// Description specifies human-friendly parameter's description
	Description() string
	Check() string
	// Required is whether the parameter is required
	Required() bool
	// Default returns the default value for the parameter
	Default() string

	// New clones this parameter
	New() Param

	// Set is required to set parameters from command line string
	Set(string) error
	// String is required to output value to command line string
	String() string

	// Args returns argument strings in cli format
	Args() []string

	// EnvVars returns a tuple with environment variable name and value
	EnvVars() (string, string)

	// Vars returns a tuple with the variable name and value
	Vars() (string, string)

	// EnvName specifies the name of the environment variable that specifies
	// the value for the parameter
	EnvName() string
}

func newParser() *cparser {
	return &cparser{
		params: []Param{},
	}
}

type cparser struct {
	params []Param
}

func (p *cparser) parse(c configV1) (*Config, error) {
	cfg := &Config{
		Params: make([]Param, len(c.Params)),
	}
	// parse types
	if len(c.Params) != 0 {
		for i, ts := range c.Params {
			pr, err := p.parseParam(ts, false)
			if err != nil {
				return nil, err
			}
			cfg.Params[i] = pr
		}
	}

	return cfg, nil
}

func (p *cparser) parseParam(s paramSpec, scalar bool) (Param, error) {
	if s.Name == "" {
		return nil, trace.Errorf("set a type name")
	}
	if err := p.checkName(s.Name); err != nil {
		return nil, err
	}
	if s.Type == "" {
		return nil, trace.Errorf("set a type for '%v'", s.Name)
	}
	switch s.Type {
	case "String":
		pr := &StringParam{}
		pr.paramCommon = s.common()
		return pr, nil
	case "Path":
		pr := &PathParam{}
		pr.paramCommon = s.common()
		return pr, nil
	case "Int":
		pr := &IntParam{}
		pr.paramCommon = s.common()
		return pr, nil
	case "Bool":
		pr := &BoolParam{}
		pr.paramCommon = s.common()
		return pr, nil
	case "KeyVal":
		return p.parseKeyVal(s)
	case "Enum":
		return p.parseEnum(s)
	case "List":
		if scalar {
			return nil, trace.Errorf(
				"scalar values are not allowed here: '%v'", s.Type)
		}
		return p.parseList(s)
	}
	return nil, trace.Errorf("unrecognized type: '%v'", s.Type)
}

func (p *cparser) parseList(s paramSpec) (Param, error) {
	var ps *paramSpec
	if err := json.Unmarshal(s.S, &ps); err != nil {
		return nil, trace.Wrap(err, "failed to parse: '%v'", string(s.S))
	}
	el, err := p.parseParam(*ps, false)
	if err != nil {
		return nil, err
	}
	l := &ListParam{el: el}
	l.paramCommon = s.common()
	return l, nil
}

func (p *cparser) parseEnum(s paramSpec) (Param, error) {
	var e *enumSpec
	if err := json.Unmarshal(s.S, &e); err != nil {
		return nil, trace.Wrap(
			err, fmt.Sprintf("failed to parse: '%v'", string(s.S)))
	}
	if len(e.Values) == 0 {
		return nil, trace.Errorf("provide at least one value for '%v'", s.Name)
	}

	values := make([]string, len(e.Values))
	seen := make(map[string]bool, len(e.Values))

	for i, v := range e.Values {
		if v == "" {
			return nil, trace.Errorf("value can not be an empty string")
		}
		if seen[v] {
			return nil, trace.Errorf("duplicate value: '%v'", v)
		}
		values[i] = v
	}

	ep := &EnumParam{values: values}
	ep.paramCommon = s.common()
	return ep, nil
}

func (p *cparser) parseKeyVal(s paramSpec) (Param, error) {
	var k *kvSpec
	if err := json.Unmarshal(s.S, &k); err != nil {
		return nil, trace.Wrap(
			err, fmt.Sprintf("failed to parse: '%v'", string(s.S)))
	}
	if len(k.Keys) == 0 {
		return nil, trace.Errorf("provide at least one key for '%v'", s.Name)
	}

	keys := make([]Param, len(k.Keys))

	for i, ks := range k.Keys {
		k, err := p.parseParam(ks, true)
		if err != nil {
			return nil, err
		}
		keys[i] = k
	}

	if err := checkSameNames(keys); err != nil {
		return nil, err
	}

	kv := &KVParam{keys: keys, separator: k.Separator}
	kv.paramCommon = s.common()
	return kv, nil
}

func (p *cparser) checkName(n string) error {
	for _, pr := range p.params {
		if pr.Name() == n {
			return trace.Errorf("parameter '%v' is already defined", n)
		}
	}
	e, err := parser.ParseExpr(n)
	if err != nil {
		return trace.Wrap(
			err, fmt.Sprintf("failed to parse name: '%v'", n))
	}
	if _, ok := e.(*ast.Ident); !ok {
		return trace.Wrap(
			err, fmt.Sprintf("name should be a valid identifier: '%v'", n))
	}
	return nil
}

func checkSameNames(ps []Param) error {
	n := map[string]bool{}
	for _, p := range ps {
		if n[p.Name()] {
			return trace.Errorf("parameter '%v' is already defined", n)
		}
		n[p.Name()] = true
	}
	return nil
}

type PathParam struct {
	paramCommon
	val *string
}

// New clones this parameter
func (p *PathParam) New() Param {
	return &PathParam{p.paramCommon, nil}
}

// Args returns argument strings in cli format
func (p *PathParam) Args() []string {
	return []string{fmt.Sprintf("--%v", p.CLIName()), p.String()}
}

// EnvVars returns a tuple with environment variable name and value
func (p *PathParam) EnvVars() (string, string) {
	return p.EnvName(), p.String()
}

// Vars returns a tuple with the variable name and value
func (p *PathParam) Vars() (string, string) {
	return p.Name(), p.String()
}

// Set is required to set parameters from command line string
func (p *PathParam) Set(s string) error {
	p.val = &s
	return nil
}

func (p *PathParam) String() string {
	if p.val == nil {
		return p.Default()
	}
	return *p.val
}

type StringParam struct {
	paramCommon
	val *string
}

// New clones this parameter
func (p *StringParam) New() Param {
	return &StringParam{p.paramCommon, nil}
}

// Set is required to set parameters from command line string
func (p *StringParam) Set(s string) error {
	p.val = &s
	return nil
}

func (p *StringParam) String() string {
	if p.val == nil {
		return p.Default()
	}
	return *p.val
}

// Args returns argument strings in cli format
func (p *StringParam) Args() []string {
	return []string{fmt.Sprintf("--%v", p.CLIName()), p.String()}
}

// EnvVars returns a tuple with environment variable name and value
func (p *StringParam) EnvVars() (string, string) {
	return p.EnvName(), p.String()
}

// Vars returns a tuple with the variable name and value
func (p *StringParam) Vars() (string, string) {
	return p.Name(), p.String()
}

type BoolParam struct {
	paramCommon
	val *bool
}

// New clones this parameter
func (p *BoolParam) New() Param {
	return &BoolParam{p.paramCommon, nil}
}

// Vars returns a tuple with the variable name and value
func (p *BoolParam) Vars() (string, string) {
	return p.Name(), p.String()
}

// Set is required to set parameters from command line string.
// Implements kingpin.Value
func (p *BoolParam) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	p.val = &v
	return nil
}

// String returns the value of this flag as text.
// Implements kingpin.Value
func (p *BoolParam) String() string {
	if p.val == nil {
		return "false"
	}
	return fmt.Sprintf("%v", *p.val)
}

// Args returns argument strings in cli format
func (p *BoolParam) Args() []string {
	if p.val == nil || !*p.val {
		return []string{fmt.Sprintf("--no-%v", p.CLIName())}
	}
	return []string{fmt.Sprintf("--%v", p.CLIName())}
}

// EnvVars returns a tuple with environment variable name and value
func (p *BoolParam) EnvVars() (string, string) {
	return p.EnvName(), p.String()
}

// IsBoolFlag marks this parameter as boolean flag
// Implements kingpin.boolFlag
func (p *BoolParam) IsBoolFlag() bool {
	return true
}

type IntParam struct {
	paramCommon
	val *int64
}

// Set is required to set parameters from command line string
func (p *IntParam) Set(s string) error {
	v, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return err
	}
	p.val = &v
	return nil
}

func (p *IntParam) String() string {
	if p.val == nil {
		return p.Default()
	}
	return fmt.Sprintf("%v", *p.val)
}

// New clones this parameter
func (p *IntParam) New() Param {
	return &IntParam{p.paramCommon, nil}
}

// Args returns argument strings in cli format
func (p *IntParam) Args() []string {
	return []string{fmt.Sprintf("--%v", p.CLIName()), p.String()}
}

// EnvVars returns a tuple with environment variable name and value
func (p *IntParam) EnvVars() (string, string) {
	return p.EnvName(), p.String()
}

// Vars returns a tuple with the variable name and value
func (p *IntParam) Vars() (string, string) {
	return p.Name(), p.String()
}

// ListParam defines a flag that accumulates multiple values
type ListParam struct {
	paramCommon
	el     Param
	values []Param
}

// CLIName specifies the name of the parameter as given on command line
func (p *ListParam) CLIName() string {
	return p.el.CLIName()
}

// EnvName specifies the name of the environment variable that specifies
// the value for the parameter
func (p *ListParam) EnvName() string {
	return p.el.EnvName()
}

// Set is required to set parameters from command line string
func (p *ListParam) Set(s string) error {
	// this is to support setting from environment variables
	values := cstrings.Split(',', '\\', s)
	for _, v := range values {
		el := p.el.New()
		if err := el.Set(v); err != nil {
			return err
		}
		p.values = append(p.values, el)
	}
	return nil
}

// New clones this parameter
func (p *ListParam) New() Param {
	return &ListParam{p.paramCommon, p.el, nil}
}

func (p *ListParam) String() string {
	if len(p.values) == 0 {
		return p.Default()
	}
	out := make([]string, len(p.values))
	for i, v := range p.values {
		out[i] = v.String()
	}
	return fmt.Sprintf("[%v]", strings.Join(out, ","))
}

// Args returns argument strings in cli format
func (p *ListParam) Args() []string {
	if len(p.values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(p.values))
	for _, v := range p.values {
		out = append(out, v.Args()...)
	}
	return out
}

// EnvVars returns a tuple with environment variable name and value
func (p *ListParam) EnvVars() (string, string) {
	if len(p.values) == 0 {
		return p.EnvName(), p.Default()
	}
	out := make([]string, len(p.values))
	for i, v := range p.values {
		_, out[i] = v.EnvVars()
	}
	return p.el.EnvName(), strings.Join(out, ",")
}

// Vars returns a tuple with the variable name and value
func (p *ListParam) Vars() (string, string) {
	if len(p.values) == 0 {
		return p.Name(), p.Default()
	}
	out := make([]string, len(p.values))
	for i, v := range p.values {
		_, out[i] = v.EnvVars()
	}
	return p.Name(), strings.Join(out, ",")
}

// IsCumulative determines whether the flag is cumulative - i.e. can be specified multiple times.
// Implements kingpin.repeatableFlag
func (p *ListParam) IsCumulative() bool {
	return true
}

type KVParam struct {
	paramCommon
	separator string
	keys      []Param
	values    []Param
}

func (p *KVParam) sep() string {
	if p.separator == "" {
		return ":"
	}
	return ""
}

// Set is required to set parameters from command line string
func (p *KVParam) Set(s string) error {
	sep := p.sep()

	parts := strings.Split(s, sep)
	if len(parts) != len(p.keys) {
		return trace.Errorf(
			"expected elements separated by '%v', got '%v'", sep, s)
	}
	values := make([]Param, len(p.keys))
	for i, pt := range parts {
		el := p.keys[i].New()
		if err := el.Set(pt); err != nil {
			return err
		}
		values[i] = el
	}

	p.values = values
	return nil
}

func (p *KVParam) String() string {
	if len(p.values) == 0 {
		return p.Default()
	}
	out := make([]string, len(p.values))
	for i, v := range p.values {
		out[i] = v.String()
	}
	return fmt.Sprintf("{%v}", strings.Join(out, p.sep()))
}

// New clones this parameter
func (p *KVParam) New() Param {
	keys := make([]Param, len(p.keys))
	for i, k := range p.keys {
		keys[i] = k.New()
	}
	return &KVParam{p.paramCommon, p.separator, keys, nil}
}

// Args returns argument strings in cli format
func (p *KVParam) Args() []string {
	if len(p.values) == 0 {
		return []string{}
	}
	vals := make([]string, len(p.values))
	for i, v := range p.values {
		vals[i] = v.String()
	}
	return []string{
		fmt.Sprintf("--%v", p.CLIName()), strings.Join(vals, p.sep())}
}

// EnvVars returns a tuple with environment variable name and value
func (p *KVParam) EnvVars() (string, string) {
	if len(p.values) == 0 {
		return p.EnvName(), p.Default()
	}
	vals := make([]string, len(p.values))
	for i, v := range p.values {
		vals[i] = v.String()
	}
	return p.EnvName(), strings.Join(vals, p.sep())
}

func (p *KVParam) Vars() (string, string) {
	if len(p.values) == 0 {
		return p.Name(), p.Default()
	}
	vals := make([]string, len(p.values))
	for i, v := range p.values {
		vals[i] = v.String()
	}
	return p.Name(), strings.Join(vals, p.sep())
}

type EnumParam struct {
	paramCommon
	values []string
	value  *string
}

// Set is required to set parameters from command line string
func (p *EnumParam) Set(s string) error {
	found := false
	for _, v := range p.values {
		if s == v {
			found = true
		}
	}
	if !found {
		return trace.Errorf(
			"value '%v' is not one of the allowed '%v'",
			s, strings.Join(p.values, ","),
		)
	}
	p.value = &s
	return nil
}

func (p *EnumParam) String() string {
	if p.value == nil {
		return p.Default()
	}
	return *p.value
}

// New clones this parameter
func (p *EnumParam) New() Param {
	return &EnumParam{p.paramCommon, p.values, nil}
}

// Args returns argument strings in cli format
func (p *EnumParam) Args() []string {
	if p.value == nil {
		return []string{}
	}
	return []string{fmt.Sprintf("--%v", p.CLIName()), *p.value}
}

// EnvVars returns a tuple with environment variable name and value
func (p *EnumParam) EnvVars() (string, string) {
	return p.EnvName(), p.String()
}

func (p *EnumParam) Vars() (string, string) {
	return p.Name(), p.String()
}
