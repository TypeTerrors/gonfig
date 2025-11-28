// options.go
package gonfig

// WithConfigFile sets the path to the YAML config file.
//
// The default is "config.yaml" in the working directory.
//
// Example:
//
//	cfg, err := gonfig.Load[Config](
//	    gonfig.WithConfigFile("config/config.yaml"),
//	)
func WithConfigFile(path string) Option {
	return func(l *loader) {
		l.configFile = path
	}
}

// WithDotenv adds a .env file to be loaded before parsing the YAML config.
//
// This is mainly useful in local development to simulate production
// environment variables.
//
// Missing .env files are ignored, so it is safe to pass a file that only
// exists on your machine.
//
// Example:
//
//	cfg, err := gonfig.Load[Config](
//	    gonfig.WithConfigFile("config/config.yaml"),
//	    gonfig.WithDotenv(".env.dev"),
//	)
func WithDotenv(path string) Option {
	return func(l *loader) {
		l.dotenvs = append(l.dotenvs, path)
	}
}

// WithStrict enables strict mode for environment variable expansion.
//
// In strict mode, any placeholder of the form ${VAR} that does not have a
// value in the environment and does not specify a default (using the
// ${VAR:-default} syntax) will cause Load to return an error.
//
// Non-strict mode (the default) replaces missing ${VAR} with an empty string.
//
// Example:
//
//	// config.yaml:
//	//   database:
//	//     password: ${DB_PASSWORD}
//
//	// main.go:
//	cfg, err := gonfig.Load[Config](
//	    gonfig.WithConfigFile("config.yaml"),
//	    gonfig.WithStrict(), // fails if DB_PASSWORD is not set
//	)
func WithStrict() Option {
	return func(l *loader) {
		l.strict = true
	}
}
