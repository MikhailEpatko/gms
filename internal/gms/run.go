package gms

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Deps struct {
	Glab     func(args ...string) ([]byte, error)
	Getenv   func(string) string
	HomeDir  func() (string, error)
	Stdout   io.Writer
	Stderr   io.Writer
	Location *time.Location
}

func Run(args []string, deps Deps) int {
	rep := reporter{w: deps.Stderr}
	if configRequested(args) {
		path, err := configPath(deps.Getenv, deps.HomeDir)
		if err != nil {
			rep.failure(err)
			return 1
		}
		_, _ = fmt.Fprintln(deps.Stdout, path)
		return 0
	}

	fs := flag.NewFlagSet("gms", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var codes codeList
	var envNames envNameList
	fs.Var(&codes, "p", "")
	fs.Var(&envNames, "e", "")
	fs.Bool("c", false, "")
	if err := fs.Parse(args); errors.Is(err, flag.ErrHelp) {
		_, _ = fmt.Fprint(deps.Stdout, usage)
		return 0
	} else if err != nil {
		rep.usageError(err)
		return 2
	}
	if fs.NArg() > 0 {
		hintCodes, hintEnvNames := hintValues(args)
		rep.extraArgs(fs.Args(), hintCodes, hintEnvNames)
		return 2
	}
	if len(codes) == 0 && flagGiven(fs, "p") {
		rep.emptyCodes()
		return 2
	}
	if len(envNames) == 0 && flagGiven(fs, "e") {
		rep.emptyEnvNames()
		return 2
	}

	path, err := configPath(deps.Getenv, deps.HomeDir)
	if err != nil {
		rep.failure(err)
		return 1
	}
	projects, err := loadConfig(path)
	if errors.Is(err, os.ErrNotExist) {
		rep.missingConfig(path)
		return 1
	}
	if err != nil {
		rep.failure(err)
		return 1
	}

	selected, unknownCodes := selectProjects(projects, codes)
	knownEnvs := knownEnvNames(projects)
	unknownEnvs := findUnknownEnvNames(knownEnvs, envNames)
	if len(unknownCodes) > 0 {
		rep.unknownCodes(projects, unknownCodes)
	}
	if len(unknownEnvs) > 0 {
		rep.unknownEnvNames(knownEnvs, unknownEnvs)
	}
	if len(unknownCodes) > 0 || len(unknownEnvs) > 0 {
		return 2
	}
	selected = selectEnvironments(selected, envNames)
	if len(selected) == 0 {
		rep.noSelectedEnvs(codes, envNames)
		return 2
	}
	var outputPairs []Environment
	for _, p := range selected {
		outputPairs = append(outputPairs, p.Environments...)
	}
	results, err := gitlab{glab: deps.Glab}.pairs(outputPairs)
	if unusable, ok := errors.AsType[*unusableError](err); ok {
		rep.glabUnusable(unusable.host, unusable.err)
		return 1
	}

	f := formatter{w: deps.Stdout, loc: deps.Location}
	anyPairFailed := false
	pairIndex := 0
	for i, p := range selected {
		if i > 0 {
			f.projectSeparator()
		}
		f.projectHeader(p.Code)
		for j, env := range p.Environments {
			if j > 0 {
				f.envSeparator()
			}
			f.pair(env, results[pairIndex])
			anyPairFailed = anyPairFailed || results[pairIndex].failed()
			pairIndex++
		}
	}
	if anyPairFailed {
		return 1
	}
	return 0
}

func configRequested(args []string) bool {
	return slices.ContainsFunc(args, func(a string) bool {
		name, value, hasValue, isFlag := flagArg(a)
		if !isFlag || name != "c" {
			return false
		}
		on, err := strconv.ParseBool(value)
		return !hasValue || err == nil && on
	})
}

func flagArg(a string) (name, value string, hasValue, isFlag bool) {
	a, isFlag = strings.CutPrefix(a, "-")
	if !isFlag {
		return "", "", false, false
	}
	name, value, hasValue = strings.Cut(strings.TrimPrefix(a, "-"), "=")
	return name, value, hasValue, true
}

type codeList []string

func (c *codeList) String() string { return strings.Join(*c, ",") }

func (c *codeList) Set(s string) error {
	*c = appendValues(*c, s, func(a, b string) bool { return a == b })
	return nil
}

type envNameList []string

func (e *envNameList) String() string { return strings.Join(*e, ",") }

func (e *envNameList) Set(s string) error {
	*e = appendValues(*e, s, sameEnvName)
	return nil
}

func appendValues(list []string, s string, same func(a, b string) bool) []string {
	for v := range strings.SplitSeq(s, ",") {
		v = strings.TrimSpace(v)
		if v == "" || slices.ContainsFunc(list, func(w string) bool { return same(v, w) }) {
			continue
		}
		list = append(list, v)
	}
	return list
}

func hintValues(args []string) (codes codeList, envNames envNameList) {
	var current flag.Value = &codes
	for i := 0; i < len(args); i++ {
		name, value, hasValue, isFlag := flagArg(args[i])
		if !isFlag {
			_ = current.Set(args[i])
			continue
		}
		switch name {
		case "p":
			current = &codes
		case "e":
			current = &envNames
		default:
			continue
		}
		nextIsValue := i+1 < len(args) && !strings.HasPrefix(args[i+1], "-")
		if !hasValue && nextIsValue {
			i++
			value = args[i]
		}
		_ = current.Set(value)
	}
	return codes, envNames
}

func flagGiven(fs *flag.FlagSet, name string) bool {
	given := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			given = true
		}
	})
	return given
}

func selectProjects(projects []Project, codes []string) (selected []Project, unknown []string) {
	if len(codes) == 0 {
		return projects, nil
	}
	for _, code := range codes {
		i := slices.IndexFunc(projects, func(p Project) bool { return p.Code == code })
		if i < 0 {
			unknown = append(unknown, code)
			continue
		}
		selected = append(selected, projects[i])
	}
	return selected, unknown
}

func selectEnvironments(projects []Project, names []string) []Project {
	if len(names) == 0 {
		return projects
	}
	var selected []Project
	for _, p := range projects {
		var envs []Environment
		for _, name := range names {
			for _, env := range p.Environments {
				if sameEnvName(env.Name, name) {
					envs = append(envs, env)
				}
			}
		}
		if len(envs) > 0 {
			p.Environments = envs
			selected = append(selected, p)
		}
	}
	return selected
}

func sameEnvName(a, b string) bool { return strings.EqualFold(a, b) }

func containsEnvName(names []string, name string) bool {
	return slices.ContainsFunc(names, func(n string) bool { return sameEnvName(n, name) })
}

func knownEnvNames(projects []Project) []string {
	var known []string
	for _, p := range projects {
		for _, env := range p.Environments {
			if !containsEnvName(known, env.Name) {
				known = append(known, env.Name)
			}
		}
	}
	return known
}

func findUnknownEnvNames(known, names []string) []string {
	var unknown []string
	for _, name := range names {
		if !containsEnvName(known, name) {
			unknown = append(unknown, name)
		}
	}
	return unknown
}
