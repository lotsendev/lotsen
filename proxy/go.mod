module github.com/ercadev/dirigent/proxy

go 1.24.0

require github.com/ercadev/dirigent/store v0.0.0

require (
	golang.org/x/crypto v0.48.0
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/text v0.34.0 // indirect
)

replace github.com/ercadev/dirigent/store => ../store
