package main

import (
	"fmt"
	"net/http"
	"os"
	"sync"

	"server/mongofns"
	"server/routes"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func welcomeFunc(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"Usage": gin.H{
			"/home":      "Welcome Page",
			"/path":      "GET request Redirects to the long URL",
			"/create/":   "POST request with path of Long URL, return short URL",
			"/path/user": "GET request Redirects the Named Short URL to long URL",
			"/named/":    "POST request with Long URL, short URL, username, email, password, expiry(hrs)",
		},
	})
}

func main() {

	r := gin.Default()
	err := godotenv.Load(".env")
	if err != nil {
		fmt.Println("Error Loading Env files")
		os.Exit(1)
	}
	client, ctx, cancel, err := mongofns.Connect(os.Getenv("MONGO"))
	if err != nil {
		fmt.Println("Error connecting to the DB: ", err)
		os.Exit(1)
	}
	defer mongofns.Close(client, ctx, cancel)

	var mt sync.Mutex

	e := routes.EnvVars{
		Client: client,
		Ctx:    ctx,
		Mt:     &mt,
	}

	r.Any("/home", welcomeFunc)
	r.POST("/create/", e.CreateShort)
	r.GET("/:path", e.GetPath)
	r.POST("/named/", e.CreateNamed)
	r.GET("/:path/:name", e.GetNamed)

	var port string
	fmt.Scanln(&port)
	port = "localhost:" + port
	r.Run(port)
}
