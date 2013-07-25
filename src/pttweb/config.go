package main

type PttwebConfig struct {
	BindAddress       string
	BoarddAddress     string
	MemcachedAddress  string
	TemplateDirectory string

	GAAccount string
	GADomain  string
}
