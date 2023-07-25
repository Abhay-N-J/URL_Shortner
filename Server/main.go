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
			"Any /":        "Welcome Page",
			"/path":        "GET request Redirects to the long URL",
			"/create/path": "POST request with path of Long URL, return short URL",
		},
	})
}

// TODO: implement route for named shortURLs

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
	// r.POST("/named", e.createNamed)
	// r.GET("/:name/:path", e.getNamed)

	r.Run("localhost:8001")
}
