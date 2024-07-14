package env

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func GetHTTPListenAddr() string {
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}
	return os.Getenv("HTTP_LISTEN_ADDR")
}

func GetMongoDBURI() string {
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}
	return os.Getenv("MONGODB_URI")
}
func GetDBApiURI() string {
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}
	return os.Getenv("DB_API_URI")
}
