module github.com/ercadev/dirigent/proxy

go 1.24.0

require (
	github.com/ercadev/dirigent/auth v0.0.0
	github.com/ercadev/dirigent/store v0.0.0
)

require (
	github.com/corazawaf/coraza-coreruleset v0.0.0-20240226094324-415b1017abdc // indirect
	github.com/corazawaf/coraza/v3 v3.3.3 // indirect
	github.com/corazawaf/libinjection-go v0.2.2 // indirect
	github.com/magefile/mage v1.15.1-0.20241126214340-bdc92f694516 // indirect
	github.com/petar-dambovaliev/aho-corasick v0.0.0-20240411101913-e07a1f0e8eb4 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/valllabh/ocsf-schema-golang v1.0.3 // indirect
	golang.org/x/sync v0.19.0 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
	rsc.io/binaryregexp v0.2.0 // indirect
)

require (
	golang.org/x/crypto v0.48.0
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/text v0.34.0 // indirect
)

replace github.com/ercadev/dirigent/auth => ../auth

replace github.com/ercadev/dirigent/store => ../store
