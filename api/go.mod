module github.com/ercadev/dirigent

go 1.24.0

require (
	github.com/ercadev/dirigent/auth v0.0.0
	github.com/ercadev/dirigent/store v0.0.0
	golang.org/x/crypto v0.48.0
)

replace github.com/ercadev/dirigent/auth => ../auth

replace github.com/ercadev/dirigent/store => ../store
