package main

type PttwebConfig struct {
	Bind              []string
	BoarddAddress     string
	MemcachedAddress  string
	TemplateDirectory string
	StaticPrefix      string

	GAAccount string
	GADomain  string
}
