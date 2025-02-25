package scan

import (
	parser "Wiggumize/internal/trafficParser"
	"sync"
)

type Scanner struct {
	ChecksMap    map[string]Check
	channel      chan channalMessage
	Results      map[string][]Finding
	chanalParams chan chanalParams
	Params       ParamsMap
}

type Check struct {
	Description string
	Execute     func(parser.HistoryItem, *Check) []Finding
	Executed    bool
	Config      interface{}
}

type Finding struct {
	Host        string
	Description string
	Evidens     string
	Details     string //TODO: add just Json implementation
}

type Result struct {
	CheckName string
	Findings  []Finding
}

type channalMessage struct {
	checkName string
	Findings  []Finding
}

type chanalParams struct {
	Host     string
	Endpoint string
	Params   EndpointParams
}

func SannerBuilder() (*Scanner, error) {
	/*
		1. Creates instance of Scanner
		2. populates ChecksMap
	*/

	// Cehck defined in the cehck's file
	var checksMap = map[string]Check{
		"Secrets":   buidSecretCheck(),
		"LFI":       buidLfiCheck(),
		"SSRF":      buidSSRFCheck(),
		"notFound":  buidNotFoundCheck(),
		"XML":       buidXMLCheck(),
		"Redirects": buidRedirectCheck(),
	}

	scanner := &Scanner{
		ChecksMap:    checksMap,
		channel:      make(chan channalMessage, 128),
		Results:      make(map[string][]Finding),
		chanalParams: make(chan chanalParams, 128),
		Params: ParamsMap{
			Name:        "Parameters",
			Description: "This module is parsing GET or POST (JSON) params",
			Hosts:       make(map[string]ParsedParams),
		},
	}

	// TODO: get list of scans from config
	return scanner, nil

}

func (s *Scanner) runChecks(p parser.HistoryItem, wg *sync.WaitGroup) {

	defer wg.Done() // signal that the worker has finished

	for key, check := range s.ChecksMap {
		findings := check.Execute(p, &check)

		s.channel <- channalMessage{
			checkName: key,
			Findings:  findings,
		}
	}

	if p.Params == "" {
		return
	}

	host, endpoint, params := parseParams(p)

	if host == "" {
		return
	}

	s.chanalParams <- chanalParams{
		Host:     host,
		Endpoint: endpoint,
		Params:   params,
	}

}

func (s *Scanner) waitForResults() {
	for {
		select {
		case msg := <-s.channel: // recived message
			s.Results[msg.checkName] = append(s.Results[msg.checkName], msg.Findings...)
		case msg := <-s.chanalParams:
			// TODO: refactor this spaggeti code
			if _, ok := s.Params.Hosts[msg.Host].Endpoints[msg.Endpoint]; !ok {

				if _, ok := s.Params.Hosts[msg.Host]; !ok {
					s.Params.Hosts[msg.Host] = ParsedParams{
						Endpoints: map[string]EndpointParams{},
					}
					s.Params.Hosts[msg.Host].Endpoints[msg.Endpoint] = msg.Params
				} else {
					s.Params.Hosts[msg.Host].Endpoints[msg.Endpoint] = msg.Params
				}
			}

			for key, val := range msg.Params.Params {
				s.Params.Hosts[msg.Host].Endpoints[msg.Endpoint].Params[key] = val
			}
		default:
		}

	}
}

func (s *Scanner) RunAllChecks(b *parser.BrowseHistory) {

	var wg sync.WaitGroup

	go s.waitForResults()

	for _, item := range b.RequestsList {

		wg.Add(1) // add a worker to the waitgroup
		go s.runChecks(item, &wg)

	}
	wg.Wait()

}
