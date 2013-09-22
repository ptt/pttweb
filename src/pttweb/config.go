package main

type PttwebConfig struct {
	BindAddress       string
	BoarddAddress     string
	MemcachedAddress  string
	TemplateDirectory string
	StaticPrefix      string

	GAAccount string
	GADomain  string
}
