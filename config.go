package main

type PttwebConfig struct {
	Bind              []string
	BoarddAddress     string
	MemcachedAddress  string
	TemplateDirectory string
	StaticPrefix      string

	BoarddMaxConn    int
	MemcachedMaxConn int

	GAAccount string
	GADomain  string

	EnableOver18Cookie bool
}
