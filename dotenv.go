// dotenv.go
package gonfig

import (
    "os"

    "github.com/joho/godotenv"
)

// loadDotenv loads a .env file into the process environment.
// Returns os.ErrNotExist if the file is missing.
func loadDotenv(path string) error {
    // godotenv returns *os.PathError for missing file
    err := godotenv.Overload(path)
    if err != nil {
        // If it's a path error, make sure we surface it as-is so
        // caller can check os.IsNotExist.
        if os.IsNotExist(err) {
            return err
        }
        return err
    }
    return nil
}