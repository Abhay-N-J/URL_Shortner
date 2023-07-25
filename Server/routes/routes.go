package routes

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type EnvVars struct {
	Client *mongo.Client
	Ctx    context.Context
	Mt     *sync.Mutex
}

type URLdoc struct {
	ID     string    `json:"_id,omitempty" bson:"_id"`
	Long   string    `json:"long" bson:"long"`
	Date   time.Time `json:"date,omitempty" bson:"date"`
	Clicks int       `josn:"clicks,omitempty" bson:"clicks"`
}

// Base62 encodes the given counter into a
// unique string and returns it

func (e *EnvVars) base62() (string, error) {

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
	collection := e.Client.Database("URL_Shortner").Collection("ShortURLsV2")
	err := collection.FindOne(e.Ctx, filter).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return result, errors.New("Create")
	}
	if doc.Date.Unix() < time.Now().Unix() {
		return result, errors.New("Update")
	}
	if err == nil {
		return e.base62()
	} else {
		return " ", err
	}
}

// createShort handles the creation of ShortURL

func (e *EnvVars) CreateShort(c *gin.Context) {

	collection := e.Client.Database("URL_Shortner").Collection("ShortURLsV2")

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
		short, err = e.base62()
		// short = "bhE9mEnnPMH"
		document.ID = short
		if err.Error() == "Update" {
			filter := bson.D{
				{Key: "_id", Value: short},
			}
			collection.DeleteOne(e.Ctx, filter)
		}
		_, err = collection.InsertOne(e.Ctx, document)

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

func (e *EnvVars) CreateNamed(c *gin.Context) {
	
}

// getPath redirects the shortURL to its respective longURL if found in DB
// TODO: Validate URLS | DONE!!, Search in PRO collection for named shortURLs

func (e *EnvVars) GetPath(c *gin.Context) {
	short := c.Param("path")
	if short == "" {
		c.Redirect(http.StatusMisdirectedRequest, "/home")
		return
	}
	collection := e.Client.Database("URL_Shortner").Collection("ShortURLsV2")

	filter := bson.D{
		{Key: "_id", Value: short},
	}

	var result URLdoc
	err := collection.FindOne(e.Ctx, filter).Decode(&result)
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
		collection.DeleteOne(e.Ctx, filter)
		c.JSON(410, gin.H{
			"message": "Link Expired",
		})
		return
	}
	opts := options.Update().SetUpsert(true)
	filter = bson.D{{Key: "_id", Value: result.ID}}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "clicks", Value: result.Clicks + 1}}}}

	collection.UpdateOne(e.Ctx, filter, update, opts)

	c.JSON(200, gin.H{
		"long": result.Long,
	})
	// c.Redirect(http.StatusMovedPermanently, result.Long)
}
