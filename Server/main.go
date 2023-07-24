package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"server/mongofns"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type EnvVars struct {
	client *mongo.Client
	ctx    context.Context
	mt     *sync.Mutex
}

type URLdoc struct {
	ID     string    `json:"_id,omitempty" bson:"_id"`
	Long   string    `json:"long" bson:"long"`
	Date   time.Time `json:"date,omitempty" bson:"date"`
	Clicks int       `josn:"clicks,omitempty" bson:"clicks"`
}

// type FormInput struct {
// 	Long string `json:"long"`
// }

// Base62 encodes the given counter into a
// unique string and returns it

func (e *EnvVars) Base62() (string, error) {

	rand.Seed(time.Now().UnixNano())
	n := rand.Int63n(1e18)

	chars := "abcedefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
	result := ""
	for n > 0 {
		result = string(chars[n%62]) + result
		n /= 62
	}
	// result = "bhE9mEnnPMH"
	filter := bson.D{
		{Key: "_id", Value: result},
	}
	var doc URLdoc
	collection := e.client.Database("URL_Shortner").Collection("ShortURLsV2")
	err := collection.FindOne(e.ctx, filter).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return result, errors.New("Create")
	}
	if doc.Date.Unix() < time.Now().Unix() {
		return result, errors.New("Update")
	}
	if err == nil {
		return e.Base62()
	} else {
		return " ", err
	}
}

// welcomeFunc gives a landing page to root path
// Specifying the usage of the API

func welcomeFunc(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"Usage": gin.H{
			"Any /":        "Welcome Page",
			"/path":        "GET request Redirects to the long URL",
			"/create/path": "POST request with path of Long URL, return short URL",
		},
	})
}

// createShort handles the creation of ShortURL
// TODO: check for already created shortURL just in case | DONE!!!

func (e *EnvVars) createShort(c *gin.Context) {

	collection := e.client.Database("URL_Shortner").Collection("ShortURLsV2")

	var document URLdoc
	var short string
	if err := c.ShouldBindJSON(&document); err != nil {
		fmt.Println(err)
		return
	}
	// path.Long = c.Param("path")
	if len(document.Long) > 10 && document.Long[0:9] != "https://" {
		document.Long = "https://" + document.Long
	} else {
		document.Long = "https://" + document.Long
	}
	fmt.Println("Link: ", document.Long)

	document.Date = time.Now().Add(time.Hour * 60)
	document.Clicks = 0

	// model := mongo.IndexModel{
	// 	Keys:    bson.M{"date": 1},
	// 	Options: options.Index().SetExpireAfterSeconds(0),
	// }
	// _, err := collection.Indexes().CreateOne(e.ctx, model)
	// if err != nil {
	// 	c.JSON(500, gin.H{
	// 		"message": "Database error",
	// 	})
	// 	fmt.Println("DB error: ", err)
	// 	return
	// }

	err := errors.New("E11000")

	for err != nil && strings.Contains(err.Error(), "E11000") {
		short, err = e.Base62()
		// short = "bhE9mEnnPMH"
		document.ID = short
		if err.Error() == "Update" {
			filter := bson.D{
				{Key: "_id", Value: short},
			}
			collection.DeleteOne(e.ctx, filter)
		}
		_, err = collection.InsertOne(e.ctx, document)

		fmt.Println("Error: ", err)
	}

	if err != nil {
		c.JSON(500, gin.H{
			"message": "Database error",
		})
		fmt.Println("DB error: ", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":  "Short URL Creation Successful",
		"shortURL": "localhost:8001/" + document.ID,
	})
}

// getPath redirects the shortURL to its respective longURL if found in DB
// TODO: Validate URLS | DONE!!, Search in PRO collection for named shortURLs

func (e *EnvVars) getPath(c *gin.Context) {
	short := c.Param("path")
	fmt.Println("FF", short)
	if short == "" {
		c.Redirect(http.StatusMisdirectedRequest, "/home")
		return
	}
	collection := e.client.Database("URL_Shortner").Collection("ShortURLsV2")

	filter := bson.D{
		{Key: "_id", Value: short},
	}

	var result URLdoc
	err := collection.FindOne(e.ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Println("Error: Document not found")
			c.JSON(404, gin.H{
				"message": "Not Found",
			})
			return
		}
		fmt.Println("Error: ", err)
		c.JSON(500, gin.H{
			"message": "DB error",
		})
		return
	}
	if result.Date.Unix() < time.Now().Unix() {
		filter = bson.D{{Key: "_id", Value: result.ID}}
		collection.DeleteOne(e.ctx, filter)
		c.JSON(410, gin.H{
			"message": "Link Expired",
		})
		return
	}
	opts := options.Update().SetUpsert(true)
	filter = bson.D{{Key: "_id", Value: result.ID}}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "clicks", Value: result.Clicks + 1}}}}

	collection.UpdateOne(e.ctx, filter, update, opts)

	// c.Redirect(http.StatusMovedPermanently, result.LongURL)
	c.JSON(200, gin.H{
		"long": result.Long,
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

	e := EnvVars{
		client: client,
		ctx:    ctx,
		mt:     &mt,
	}

	r.Any("/home", welcomeFunc)
	r.POST("/create/", e.createShort)
	r.GET("/:path", e.getPath)

	r.Run("localhost:8001")
}
