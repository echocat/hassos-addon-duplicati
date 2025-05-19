package main

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	separatorRegexp = regexp.MustCompile("[.:_-]+")

	defaultProperties = properties{
		"webservice-disable-https":     "True",
		"log-file":                     "/dev/stdout",
		"log-level":                    "Information",
		"webservice-interface":         "any",
		"webservice-allowed-hostnames": "*",
		"server-datafolder":            "/data",
		"require-db-encryption-key":    "True",
		"webservice-timezone":          "Etc/UTC",
		"disable-tray-icon-login":      "True",
		"startup-delay":                "5m",
		"update-channel":               "stable",
	}
)

type properties map[string]string

func (p properties) merge(with properties) properties {
	result := make(properties, len(p))
	for k, v := range p {
		result[p.normalizeKey(k)] = strings.Clone(v)
	}
	for k, v := range with {
		result[p.normalizeKey(k)] = strings.Clone(v)
	}
	return result
}

func (p properties) setMap(v map[string]any) error {
	for k, v := range v {
		k = p.normalizeKey(k)
		switch vv := v.(type) {
		case string:
			p[k] = strings.Clone(vv)
		case *string:
			p[k] = strings.Clone(*vv)
		case bool:
			if vv {
				p[k] = "True"
			} else {
				p[k] = "False"
			}
		case *bool:
			if *vv {
				p[k] = "True"
			} else {
				p[k] = "False"
			}
		default:
			p[k] = fmt.Sprint(v)
		}
	}
	return nil
}

func (p properties) clone() properties {
	result := make(properties, len(p))
	for k, v := range p {
		result[strings.Clone(k)] = strings.Clone(v)
	}
	return result
}

func (p properties) with(k, v string) properties {
	result := p.clone()
	result[p.normalizeKey(k)] = strings.Clone(v)
	return result
}

func (p properties) toArguments() []string {
	result := make([]string, len(p))
	i := 0
	for k, v := range p {
		result[i] = fmt.Sprintf("--%s=%s", p.normalizeKey(k), v)
		i++
	}
	return result
}

func (p properties) normalizeKey(in string) string {
	result := in
	result = strings.ToLower(result)
	result = separatorRegexp.ReplaceAllString(result, "-")
	result = strings.TrimPrefix(result, "duplicati-")
	return result
}
