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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type EnvVars struct {
	Client *mongo.Client
	Ctx    context.Context
	Mt     *sync.Mutex
}

type URLdoc struct {
	ID           string    `json:"_id,omitempty" bson:"_id"`
	Long         string    `json:"long" bson:"long"`
	ExpiryDate   time.Time `json:"expiry,omitempty" bson:"expiry"`
	CreationDate time.Time `json:"creation,omitempty" bson:"creation"`
	Clicks       int       `josn:"clicks,omitempty" bson:"clicks"`
}

type URL struct {
	Long       string
	ExpiryDate time.Time
	Clicks     int
}

type USERdoc struct {
	ID         string         `json:"user" bson:"_id"`
	Email      string         `json:"email" bson:"email"`
	Passwd     string         `json:"pass" bson:"passwd"`
	URLs       map[string]URL `json:"urls,omitempty" bson:"urls"`
	Short      string         `json:"short" bson:"-"`
	Long       string         `json:"long" bson:"-"`
	ExpiryDays int            `json:"expiry" bson:"-"`
}

// base62 encodes the given counter into a
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
	if doc.ExpiryDate.Unix() < time.Now().Unix() {
		return result, errors.New("UpExpiryDate")
	}
	if err == nil {
		return e.base62()
	} else {
		return " ", err
	}
}

// CreateShort handles the creation of ShortURL

func (e *EnvVars) CreateShort(c *gin.Context) {

	collection := e.Client.Database("URL_Shortner").Collection("ShortURLsV2")

	var document URLdoc
	var short string
	if err := c.ShouldBindJSON(&document); err != nil {
		fmt.Println(err)
		c.JSON(400, gin.H{
			"error": "Wrong input",
		})
		return
	}
	// path.Long = c.Param("path")
	if len(document.Long) > 10 && document.Long[0:9] != "https://" {
		document.Long = "https://" + document.Long
	} else {
		document.Long = "https://" + document.Long
	}
	fmt.Println("Link: ", document.Long)

	document.ExpiryDate = time.Now().Add(time.Hour * 60)
	document.CreationDate = time.Now()
	document.Clicks = 0

	// model := mongo.IndexModel{
	// 	Keys:    bson.M{"ExpiryDate": 1},
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
		if err.Error() == "UpExpiryDate" {
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

// TODO: passwd hashing
// CreateNamed handles the creation of Custom short URLs

func (e *EnvVars) CreateNamed(c *gin.Context) {
	collection := e.Client.Database("URL_Shortner").Collection("NamedURLsV2")

	var document USERdoc
	var searchDoc USERdoc
	err := c.ShouldBindJSON(&document)
	if err != nil {
		fmt.Println(err)
		c.JSON(400, gin.H{
			"error": "Wrong input",
		})
		return
	}

	if len(document.Long) > 10 && document.Long[0:9] != "https://" {
		document.Long = "https://" + document.Long
	} else {
		document.Long = "https://" + document.Long
	}
	fmt.Println("Link: ", document.Long)

	filter := bson.D{{Key: "_id", Value: document.ID}}
	err = collection.FindOne(e.Ctx, filter).Decode(&searchDoc)
	ok := false
	if err == nil {
		_, ok = searchDoc.URLs[document.Short]
		fmt.Println(ok)
	}
	// Creation of user is handled here
	if err == mongo.ErrNoDocuments {
		document.URLs = make(map[string]URL)
		document.URLs[document.Short] = URL{
			Long:       document.Long,
			ExpiryDate: time.Now().Add(time.Hour * 24 * time.Duration(document.ExpiryDays)),
			Clicks:     0,
		}
		_, err = collection.InsertOne(e.Ctx, document)
	} else if err == nil && ok {
		c.JSON(400, gin.H{
			"error": "URL Already exists",
		})
		return
	} else if err == nil {
		searchDoc.URLs[document.Short] = URL{
			Long:       document.Long,
			ExpiryDate: time.Now().Add(time.Hour * 24 * time.Duration(document.ExpiryDays)),
			Clicks:     0,
		}
		opts := options.Update().SetUpsert(true)
		filter = bson.D{{Key: "_id", Value: searchDoc.ID}}
		update := bson.D{{Key: "$set", Value: bson.D{{Key: "urls", Value: searchDoc.URLs}}}}
		_, err = collection.UpdateOne(e.Ctx, filter, update, opts)
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
		"shortURL": "localhost:8001/" + document.ID + "/" + document.Short,
	})

}

// GetNamed fetches the Long URL for custom short urls

func (e *EnvVars) GetNamed(c *gin.Context) {
	short := c.Param("path")
	user := c.Param("name")

	// if user != "" {
	// 	go e.GetPath(c)
	// 	return
	// }
	fmt.Println("Short", short)
	collection := e.Client.Database("URL_Shortner").Collection("NamedURLsV2")

	filter := bson.D{
		{Key: "_id", Value: user},
	}

	var result USERdoc
	err := collection.FindOne(e.Ctx, filter).Decode(&result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Println("Error: Page not found")
			c.JSON(404, gin.H{
				"message": "User and Page Not Found",
			})
			return
		}
		fmt.Println("Error: ", err)
		c.JSON(500, gin.H{
			"message": "DB error",
		})
		return
	}
	opts := options.Update().SetUpsert(true)
	filter = bson.D{{Key: "_id", Value: result.ID}}
	var update primitive.D
	entry, ok := result.URLs[short]
	if !ok {
		c.JSON(404, gin.H{
			"message": "Page Not Found",
		})
		return
	}
	if result.URLs[short].ExpiryDate.Unix() < time.Now().Unix() {
		delete(result.URLs, short)
		c.JSON(410, gin.H{
			"message": "Link Expired",
		})
	} else {
		entry.Clicks += 1
		result.URLs[short] = entry
	}
	update = bson.D{{Key: "$set", Value: bson.D{{Key: "urls", Value: result.URLs}}}}
	collection.UpdateOne(e.Ctx, filter, update, opts)

	c.JSON(200, gin.H{
		"long": result.URLs[short].Long,
	})
	// c.Redirect(http.StatusMovedPermanently, result.Long)

}

// GetPath redirects the shortURL to its respective longURL if found in DB

func (e *EnvVars) GetPath(c *gin.Context) {
	short := c.Param("path")

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
	if result.ExpiryDate.Unix() < time.Now().Unix() {
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
