module github.com/deploymenttheory/go-microsoft-dm/server

go 1.27.0

require github.com/deploymenttheory/go-microsoft-dm v0.0.0

// Resolve the library from this checkout when building the server module directly.
replace github.com/deploymenttheory/go-microsoft-dm => ../
