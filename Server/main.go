package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// connect is a userdefined function returns mongo.Client,
// context.Context, context.CancelFunc and error.
// context.Context is used to set a deadline for process
// context.CancelFunc is used to cancel context and its resources

func connect(uri string) (*mongo.Client, context.Context,
	context.CancelFunc, error) {

	ctx, cancel := context.WithCancel(context.Background())
	// ctx, cancel := context.WithTimeout(context.Background())
	// 30*time.Second)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	return client, ctx, cancel, err
}

// close is a userdef func to close resources
// closes MongoDB connection and cancel context

func close(client *mongo.Client, ctx context.Context,
	cancel context.CancelFunc) {

	defer cancel()
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()
}

// welcomeFunc gives a landing page to root path
// Specifying the usage of the API

func welcomeFunc(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"Usage": gin.H{
			"/":            "Welcome Page",
			"/path":        "Redirects to the long URL",
			"/create/path": "Get request with path of Long URL, return short URL",
		},
	})
}

type EnvVars struct {
	client  *mongo.Client
	ctx     context.Context
	initial int
	counter int
	end     int
	mt      *sync.Mutex
}

type URLdoc struct {
	// ID     primitive.ObjectID
	ShortURL string `bson:"short"`
	LongURL  string `bson:"long"`
	Date     string `bson:"date"`
	Clicks   int    `bson:"clicks"`
}

// Base62 encodes the given counter into a
// unique string and returns it

func (e *EnvVars) Base62() string {
	e.mt.Lock()
	n := e.counter
	e.counter += 1
	e.mt.Unlock()

	chars := "abcedefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
	result := ""
	for n > 0 {
		result = string(chars[n%62]) + result
		n /= 62
	}
	return result
}

// createShort handles the creation of ShortURL
// TODO: check for already created shortURL just in case

func (e *EnvVars) createShort(c *gin.Context) {

	collection := e.client.Database("URL_Shortner").Collection("ShortURLs")

	path := c.Param("path")
	if path[0:9] != "https://" {
		path = "https://" + path
	}
	short := e.Base62()

	fmt.Println(e.counter)
	document := bson.D{
		{Key: "short", Value: short},
		{Key: "long", Value: path},
		{Key: "date", Value: time.DateTime},
		{Key: "clicks", Value: 0},
	}

	_, err := collection.InsertOne(e.ctx, document)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "Database error",
		})
		fmt.Println("DB error: ", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":  "Short URL Creation Successful",
		"shortURL": "localhost:8001/" + short,
	})
}

// getPath redirects the shortURL to its respective longURL if found in DB
// TODO: Validate URLS, Search in PRO collection for named shortURLs

func (e *EnvVars) getPath(c *gin.Context) {
	short := c.Param("path")

	collection := e.client.Database("URL_Shortner").Collection("ShortURLs")

	filter := bson.D{
		{Key: "short", Value: short},
	}

	var result URLdoc
	err := collection.FindOne(e.ctx, filter).Decode(&result)
	// doc := collection.FindOne(e.ctx, filter)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Println("Error: Document not found")
			c.JSON(403, gin.H{
				"message": "Bad request",
			})
			return
		}
		fmt.Println("Error: ", err)
		c.JSON(500, gin.H{
			"message": "DB error",
		})
		return
	}

	c.Redirect(http.StatusMovedPermanently, result.LongURL)
	// c.JSON(200, gin.H{
	// 	"long": result.LongURL,
	// })
}

// TODO: implement route for named shortURLs, get range from Zookeeper and validate URL range (cycle back to initial, get another range or another method)

func main() {

	r := gin.Default()
	err := godotenv.Load(".env")
	if err != nil {
		fmt.Println("Error Loading Env files")
		os.Exit(1)
	}
	client, ctx, cancel, err := connect(os.Getenv("MONGO"))
	if err != nil {
		fmt.Println("Error connecting to the DB: ", err)
		os.Exit(1)
	}
	defer close(client, ctx, cancel)

	// initial, end := Zookeeper()
	initial, end, counter := 91574294826053, 915742948260530, 91574294826053

	var mt sync.Mutex

	e := EnvVars{
		client:  client,
		ctx:     ctx,
		initial: initial,
		counter: counter,
		end:     end,
		mt:      &mt,
	}

	r.Any("/", welcomeFunc)
	r.GET("/create/:path", e.createShort)
	r.GET("/link/:path", e.getPath)

	r.Run("localhost:8001")
}
